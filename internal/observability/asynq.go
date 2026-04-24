package observability

import (
	"context"
	"fmt"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/otelattr"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// InjectIntoTaskHeaders reads the active trace from the context and serializes it into a string map
// that can be passed to asynq.NewTaskWithHeaders(). This allows the trace to travel across Redis.
func InjectIntoTaskHeaders(ctx context.Context) map[string]string {
	headers := make(map[string]string)
	carrier := propagation.MapCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return headers
}

// AsynqTracingMiddleware is a server-side middleware for Asynq workers.
// It intercepts incoming jobs, extracts the trace context from the headers, and sets up a child span.
func AsynqTracingMiddleware(serviceName string) asynq.MiddlewareFunc {
	// The scope outside return is cached and reused across all tasks.
	tracer := otel.Tracer(serviceName)
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			// Extract trace context from task headers (deserializes W3C headers into Go context)
			headers := task.Headers()
			if headers == nil {
				headers = make(map[string]string)
			}
			carrier := propagation.MapCarrier(headers)
			ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

			// Start a span for the worker execution
			spanName := fmt.Sprintf("Worker Process %s", task.Type())
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindConsumer),
				trace.WithAttributes(
					semconv.MessagingSystemKey.String("asynq"),
					semconv.MessagingOperationTypeProcess,
					otelattr.TaskType(task.Type()),
				),
			)
			defer span.End()
			if taskID, ok := asynq.GetTaskID(ctx); ok {
				span.SetAttributes(semconv.MessagingMessageID(taskID))
			}
			if queue, ok := asynq.GetQueueName(ctx); ok {
				span.SetAttributes(semconv.MessagingDestinationName(queue))
			}

			// Inject Task type for logs
			ctx = appctx.WithTaskType(ctx, task.Type())

			// Execute actual task handler
			if err := next.ProcessTask(ctx, task); err != nil {
				span.RecordError(err)
				span.SetStatus(
					codes.Error,
					fmt.Sprintf("Failed to process task %s", task.Type()),
				)
				return err
			}

			return nil
		})
	}
}

// AsynqProducerAttributes returns the standard OTel attributes for publishing to an Asynq queue.
func AsynqProducerAttributes(taskType string, queue string) []trace.SpanStartOption {
	return []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("asynq"),
			semconv.MessagingOperationTypeSend,
			semconv.MessagingDestinationName(queue),
			otelattr.TaskType(taskType),
		),
	}
}
