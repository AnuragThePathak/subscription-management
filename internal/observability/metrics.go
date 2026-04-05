package observability

import (
	"context"
	"fmt"

	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// OTelMetricsAdapter bridges the strictly typed domain metrics interface
// to the external OpenTelemetry Prometheus engine dynamically constructed from YAML configuration.
type OTelMetricsAdapter struct {
	created metric.Int64Counter
	canceled metric.Int64Counter
	active   metric.Int64UpDownCounter
}

// NewMetricsAdapter generates the OpenTelemetry adapter with dynamic
// metric names and descriptions sourced from the configuration variables.
func NewMetricsAdapter(cfg Config) (services.SubscriptionMetrics, error) {
	meter := otel.Meter(cfg.ServiceName)

	createdCounter, err := meter.Int64Counter(
		cfg.Metrics.SubscriptionsCreatedCount.Name,
		metric.WithDescription(cfg.Metrics.SubscriptionsCreatedCount.Description),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'subscriptions_created' metric counter: %w", err)
	}

	canceledCounter, err := meter.Int64Counter(
		cfg.Metrics.SubscriptionsCanceledCount.Name,
		metric.WithDescription(cfg.Metrics.SubscriptionsCanceledCount.Description),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'subscriptions_canceled' metric counter: %w", err)
	}

	activeUpDown, err := meter.Int64UpDownCounter(
		cfg.Metrics.ActiveSubscriptionsCount.Name,
		metric.WithDescription(cfg.Metrics.ActiveSubscriptionsCount.Description),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'active_subscriptions' metric updown counter: %w", err)
	}

	return &OTelMetricsAdapter{
		created:  createdCounter,
		canceled: canceledCounter,
		active:   activeUpDown,
	}, nil
}

func (o *OTelMetricsAdapter) IncSubscriptionsCreated(ctx context.Context) {
	o.created.Add(ctx, 1)
}

func (o *OTelMetricsAdapter) IncSubscriptionsCanceled(ctx context.Context) {
	o.canceled.Add(ctx, 1)
}

func (o *OTelMetricsAdapter) IncActiveSubscriptions(ctx context.Context) {
	o.active.Add(ctx, 1)
}

func (o *OTelMetricsAdapter) DecActiveSubscriptions(ctx context.Context) {
	o.active.Add(ctx, -1)
}

// NoOpMetricsAdapter is a dummy implementation used when telemetry is disabled,
// ensuring the domain service never panics on a nil interface call.
type NoOpMetricsAdapter struct{}

func NewNoOpMetricsAdapter() services.SubscriptionMetrics {
	return &NoOpMetricsAdapter{}
}

func (n *NoOpMetricsAdapter) IncSubscriptionsCreated(ctx context.Context)  {}
func (n *NoOpMetricsAdapter) IncSubscriptionsCanceled(ctx context.Context) {}
func (n *NoOpMetricsAdapter) IncActiveSubscriptions(ctx context.Context)   {}
func (n *NoOpMetricsAdapter) DecActiveSubscriptions(ctx context.Context)   {}
