package observability

import (
	"context"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/hibiken/asynq"
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
	record = record.Clone()

	// Add user ID to the log record if available
	if userID, ok := appctx.GetUserID(ctx); ok {
		record.AddAttrs(logattr.UserID(userID))
	}

	// Add subscription ID to the log record if available
	if subscriptionID, ok := appctx.GetSubscriptionID(ctx); ok {
		record.AddAttrs(logattr.SubscriptionID(subscriptionID))
	}

	// Add task type to the log record if available
	if taskType, ok := appctx.GetTaskType(ctx); ok {
		record.AddAttrs(logattr.TaskType(taskType))
	}

	// Add task ID to the log record if available
	if taskID, ok := asynq.GetTaskID(ctx); ok {
		record.AddAttrs(logattr.TaskID(taskID))
	}

	// Add trace ID and span ID to the log record if available
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		record.AddAttrs(
			logattr.TraceID(spanCtx.TraceID().String()),
			logattr.SpanID(spanCtx.SpanID().String()),
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
