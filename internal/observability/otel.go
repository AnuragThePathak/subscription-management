package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider holds the initialized OTel providers and exposes a shutdown method.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
}

// InitOTel initializes OpenTelemetry with trace and metric providers.
// It returns a Provider whose Shutdown method should be deferred.
func InitOTel(ctx context.Context, cfg Config) (*Provider, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Trace exporter: OTLP gRPC → Jaeger.
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.JaegerEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(tracerProvider)

	// Propagator: W3C Trace Context + Baggage for cross-service propagation.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Metrics exporter: Prometheus pull-based.
	prometheusExporter, err := otelprometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(prometheusExporter),
	)
	otel.SetMeterProvider(meterProvider)

	slog.Info("OpenTelemetry initialized",
		logattr.Service(cfg.ServiceName),
		logattr.Jaeger(cfg.JaegerEndpoint),
	)

	return &Provider{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
	}, nil
}

// Shutdown flushes and shuts down all OTel providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down OpenTelemetry providers")

	var errs []error
	if err := p.tracerProvider.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown tracer provider",
			logattr.Error(err),
		)
		errs = append(errs, err)
	}

	if err := p.meterProvider.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown meter provider",
			logattr.Error(err),
		)
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		slog.Info("OpenTelemetry shut down successfully")
	}
	return errors.Join(errs...)
}
