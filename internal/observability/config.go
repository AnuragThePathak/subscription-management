package observability

// MetricConfig encapsulates the metadata for a single telemetry metric.
type MetricConfig struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
}

// Config holds the configuration needed to initialize OpenTelemetry.
type Config struct {
	Enabled        bool   `mapstructure:"enabled"`      // Enable OpenTelemetry instrumentation.
	ServiceName    string `mapstructure:"service_name"` // Service name for traces and metrics.
	Environment    string // Environment injected by main application config (not mapped from yaml).
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"` // OTLP gRPC endpoint for Jaeger.
	Metrics        struct {
		SubscriptionsCreatedCount  MetricConfig `mapstructure:"subscriptions_created_count"`
		SubscriptionsCanceledCount MetricConfig `mapstructure:"subscriptions_canceled_count"`
		ActiveSubscriptionsCount   MetricConfig `mapstructure:"active_subscriptions_count"`
	} `mapstructure:"metrics"`
}
