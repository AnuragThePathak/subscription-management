package observability

import (
	"context"

	"github.com/anuragthepathak/subscription-management/internal/lib"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// EnrichContext attaches core business entity IDs and task identifiers to the active OpenTelemetry span,
// and injects them into the Go context so the custom slog handler can automatically log them.
func EnrichContext(ctx context.Context, userID, subscriptionID string) context.Context {
	// Inject into the Go context for slog
	ctx = lib.WithUserID(ctx, userID)
	ctx = lib.WithSubscriptionID(ctx, subscriptionID)

	return ctx
}

func EnrichSpan(ctx context.Context) {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		if userID, ok := lib.GetUserID(ctx); ok {
			span.SetAttributes(semconv.EnduserID(userID))
		}
		if subscriptionID, ok := lib.GetSubscriptionID(ctx); ok {
			span.SetAttributes(SubscriptionID(subscriptionID))
		}
	}
}