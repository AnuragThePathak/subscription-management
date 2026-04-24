package observability

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// OTelMetricsAdapter bridges the strictly typed domain metrics interface
// to the external OpenTelemetry Prometheus engine dynamically constructed from YAML configuration.
type OTelMetricsAdapter struct {
	created  metric.Int64Counter
	canceled metric.Int64Counter
}

// stateProvider defines the exact data the metrics adapter needs from the outside world.
type stateProvider interface {
	CountActiveSubscriptions(ctx context.Context) (int64, error)
}

// NewMetricsAdapter generates the OpenTelemetry adapter with dynamic
// metric names and descriptions sourced from the configuration variables.
func NewMetricsAdapter(cfg Config, state stateProvider) (*OTelMetricsAdapter, error) {
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

	activeGauge, err := meter.Int64ObservableGauge(
		cfg.Metrics.ActiveSubscriptionsCount.Name,
		metric.WithDescription(cfg.Metrics.ActiveSubscriptionsCount.Description),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'active_subscriptions' metric updown counter: %w", err)
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		activeSubscriptionsCount, subscriptionErr := state.CountActiveSubscriptions(ctx)
		if subscriptionErr != nil {
			slog.ErrorContext(ctx,
				"Failed to fetch active subscriptions count for telemetry",
				logattr.Error(subscriptionErr),
			)
			return nil
		}
		o.ObserveInt64(activeGauge, activeSubscriptionsCount)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register callback for 'active_subscriptions' metric: %w", err)
	}

	return &OTelMetricsAdapter{
		created:  createdCounter,
		canceled: canceledCounter,
	}, nil
}

func (o *OTelMetricsAdapter) IncSubscriptionsCreated(ctx context.Context) {
	o.created.Add(ctx, 1)
}

func (o *OTelMetricsAdapter) IncSubscriptionsCanceled(ctx context.Context) {
	o.canceled.Add(ctx, 1)
}

// NewNoOpMetricsAdapter returns an *OTelMetricsAdapter backed by OTel's
// built-in noop instruments. All method calls are safe no-ops, keeping the
// domain layer free of nil checks while avoiding a separate type.
func NewNoOpMetricsAdapter() *OTelMetricsAdapter {
	meter := noop.NewMeterProvider().Meter("noop")
	created, _ := meter.Int64Counter("noop")
	canceled, _ := meter.Int64Counter("noop")
	return &OTelMetricsAdapter{
		created:  created,
		canceled: canceled,
	}
}
