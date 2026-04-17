package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

type MongoMetricsMonitor struct {
	histogram metric.Float64Histogram
	starts    sync.Map
}

func NewMongoMetricsMonitor() *event.CommandMonitor {
	meter := otel.Meter("mongo-driver")
	hist, err := meter.Float64Histogram(
		"db.client.operation.duration",
		metric.WithDescription("MongoDB query duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Error("failed to create db.client.operation.duration histogram", logattr.Error(err))
		return &event.CommandMonitor{}
	}

	m := &MongoMetricsMonitor{
		histogram: hist,
	}

	return &event.CommandMonitor{
		Started:   m.Started,
		Succeeded: m.Succeeded,
		Failed:    m.Failed,
	}
}

func (m *MongoMetricsMonitor) Started(ctx context.Context, evt *event.CommandStartedEvent) {
	m.starts.Store(evt.RequestID, time.Now())
}

func (m *MongoMetricsMonitor) Succeeded(ctx context.Context, evt *event.CommandSucceededEvent) {
	if start, ok := m.starts.LoadAndDelete(evt.RequestID); ok {
		duration := time.Since(start.(time.Time)).Seconds()
		m.histogram.Record(ctx, duration, metric.WithAttributes(
			semconv.DBSystemNameMongoDB,
			semconv.DBOperationNameKey.String(evt.CommandName),
			attribute.String("status", "success"),
		))
	}
}

func (m *MongoMetricsMonitor) Failed(ctx context.Context, evt *event.CommandFailedEvent) {
	if start, ok := m.starts.LoadAndDelete(evt.RequestID); ok {
		duration := time.Since(start.(time.Time)).Seconds()
		m.histogram.Record(ctx, duration, metric.WithAttributes(
			semconv.DBSystemNameMongoDB,
			semconv.DBOperationNameKey.String(evt.CommandName),
			attribute.String("status", "failed"),
		))
	}
}
