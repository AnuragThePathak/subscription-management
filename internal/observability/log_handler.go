package observability

import (
	"context"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/lib"
	"go.opentelemetry.io/otel/trace"
)

// traceHandler wraps an slog.Handler to inject trace_id and span_id
// from the OpenTelemetry span context into every log record.
//
// When no active span exists in the context, the handler passes through
// to the underlying handler without adding any trace fields.
type traceHandler struct {
	inner slog.Handler
}

// NewTraceHandler wraps an existing slog.Handler with trace correlation.
func NewTraceHandler(inner slog.Handler) slog.Handler {
	return &traceHandler{inner: inner}
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, record slog.Record) error {
	// Add user ID to the log record if available
	if userID, ok := lib.GetUserID(ctx); ok {
		record.AddAttrs(slog.String("user_id", userID))
	}

	// Add subscription ID to the log record if available
	if subscriptionID, ok := lib.GetSubscriptionID(ctx); ok {
		record.AddAttrs(slog.String("subscription_id", subscriptionID))
	}

	// Add trace ID and span ID to the log record if available
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}

	return h.inner.Handle(ctx, record)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}
