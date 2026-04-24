package observability

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/otelattr"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func InitQueueMetrics(serviceName string, redisConfig asynq.RedisConnOpt) error {
	meter := otel.Meter(serviceName)

	// Create the Gauge for Queue Depth
	queueDepthGauge, err := meter.Int64ObservableGauge(
		"worker.queue.depth",
		metric.WithDescription("Number of tasks currently in the queue by state"),
	)
	if err != nil {
		return fmt.Errorf("failed to create queue depth gauge: %w", err)
	}

	inspector := asynq.NewInspector(redisConfig)

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		queues, qErr := inspector.Queues()
		if qErr != nil {
			slog.ErrorContext(ctx, "Failed to fetch asynq queues for telemetry",
				logattr.Error(qErr))
			return nil // Return nil so we don't crash the scrape, just leave gaps
		}

		for _, q := range queues {
			info, infoErr := inspector.GetQueueInfo(q)
			if infoErr != nil {
				continue
			}

			metricAttrs := []attribute.KeyValue{
				semconv.MessagingSystemKey.String("asynq"),
				semconv.MessagingDestinationName(q),
			}
			// Completed: Tasks that succeeded but haven't been garbage collected yet
			o.ObserveInt64(queueDepthGauge, int64(info.Completed),
				metric.WithAttributes(append(metricAttrs, otelattr.StateCompleted)...),
			)
			// Active: Tasks currently being executed by workers
			o.ObserveInt64(queueDepthGauge, int64(info.Active),
				metric.WithAttributes(append(metricAttrs, otelattr.StateActive)...),
			)
			// Pending: Tasks waiting to be processed right now
			o.ObserveInt64(queueDepthGauge, int64(info.Pending),
				metric.WithAttributes(append(metricAttrs, otelattr.StatePending)...),
			)
			// Scheduled: Tasks scheduled for future execution
			o.ObserveInt64(queueDepthGauge, int64(info.Scheduled),
				metric.WithAttributes(append(metricAttrs, otelattr.StateScheduled)...),
			)
			// Retry: Tasks scheduled for retry
			o.ObserveInt64(queueDepthGauge, int64(info.Retry),
				metric.WithAttributes(append(metricAttrs, otelattr.StateRetry)...),
			)
			// Archived: Tasks that failed persistently
			o.ObserveInt64(queueDepthGauge, int64(info.Archived),
				metric.WithAttributes(append(metricAttrs, otelattr.StateArchived)...),
			)
		}

		return nil
	}, queueDepthGauge)

	if err != nil {
		return fmt.Errorf("failed to initialize queue metrics: %w", err)
	}
	return nil
}
