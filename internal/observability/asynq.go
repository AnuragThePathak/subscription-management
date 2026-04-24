package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/otelattr"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
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
	meter := otel.Meter(serviceName)

	taskDuration, err := meter.Float64Histogram(
		"worker.task.duration_seconds",
		metric.WithDescription("Duration of background tasks in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Error("Failed to create task duration histogram",
			logattr.WorkerName(serviceName),
			logattr.Error(err),
		)
	}
	taskCount, err := meter.Int64Counter(
		"worker.task.total",
		metric.WithDescription("Total number of background tasks processed"),
	)
	if err != nil {
		slog.Error("Failed to create task count counter",
			logattr.WorkerName(serviceName),
			logattr.Error(err),
		)
	}

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			// Extract trace context from task headers (deserializes W3C headers into Go context)
			headers := task.Headers()
			if headers == nil {
				headers = make(map[string]string)
			}
			carrier := propagation.MapCarrier(headers)
			ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
			// Inject Task type for logs
			ctx = appctx.WithTaskType(ctx, task.Type())

			otelAttrs := []attribute.KeyValue{
				semconv.MessagingSystemKey.String("asynq"),
				semconv.MessagingOperationTypeProcess,
				otelattr.TaskType(task.Type()),
			}
			// Start a span for the worker execution
			spanName := fmt.Sprintf("Worker Process %s", task.Type())
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindConsumer),
			)
			defer span.End()
			if taskID, ok := asynq.GetTaskID(ctx); ok {
				otelAttrs = append(otelAttrs, semconv.MessagingMessageID(taskID))
			}
			if queue, ok := asynq.GetQueueName(ctx); ok {
				otelAttrs = append(otelAttrs, semconv.MessagingDestinationName(queue))
			}
			span.SetAttributes(otelAttrs...)

			// Execute actual task handler
			start := time.Now()
			err := next.ProcessTask(ctx, task)
			duration := time.Since(start).Seconds()
			if err != nil {
				span.RecordError(err)
				span.SetStatus(
					codes.Error,
					fmt.Sprintf("Failed to process task %s", task.Type()),
				)
				otelAttrs = append(otelAttrs, otelattr.StatusError)
			} else {
				otelAttrs = append(otelAttrs, otelattr.StatusSuccess)
			}

			metricOptions := metric.WithAttributes(otelAttrs...)
			taskDuration.Record(ctx, duration, metricOptions)
			taskCount.Add(ctx, 1, metricOptions)
			return err
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
