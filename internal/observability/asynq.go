package observability

import (
	"context"
	"fmt"

	"github.com/anuragthepathak/subscription-management/internal/lib"
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
					semconv.MessagingOperationName("process"),
					semconv.MessagingDestinationNameKey.String(task.Type()),
				),
			)
			defer span.End()

			// Inject Task ID for trace span
			if taskID, ok := asynq.GetTaskID(ctx); ok {
				span.SetAttributes(TaskID(taskID))
			}

			// Inject Task type for logs
			ctx = lib.WithTaskType(ctx, task.Type())

			// Execute actual task handler
			err := next.ProcessTask(ctx, task)

			// Record failure if it errored
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return err
		})
	}
}

// AsynqProducerAttributes returns the standard OTel attributes for publishing to an Asynq queue.
func AsynqProducerAttributes(taskName string) []trace.SpanStartOption {
	return []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("asynq"),
			semconv.MessagingOperationName("publish"),
			semconv.MessagingDestinationNameKey.String(taskName),
		),
	}
}
