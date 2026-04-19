package observability

import (
	"context"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/traceattr"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// EnrichContext attaches core business entity IDs and task identifiers to the active OpenTelemetry span,
// and injects them into the Go context so the custom slog handler can automatically log them.
func EnrichContext(ctx context.Context, userID, subscriptionID string) context.Context {
	// Inject into the Go context for slog
	ctx = appctx.WithUserID(ctx, userID)
	ctx = appctx.WithSubscriptionID(ctx, subscriptionID)

	return ctx
}

func EnrichSpan(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	if userID, ok := appctx.GetUserID(ctx); ok {
		span.SetAttributes(semconv.EnduserID(userID))
	}
	if subscriptionID, ok := appctx.GetSubscriptionID(ctx); ok {
		span.SetAttributes(traceattr.SubscriptionID(subscriptionID))
	}
}
