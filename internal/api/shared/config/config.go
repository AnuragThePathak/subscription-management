package config

import (
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/notifications"
)

// ServerConfig holds the server configuration, including TLS settings.
type ServerConfig struct {
	Port           int           `mapstructure:"port"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	TLS            struct {
		Enabled  bool   `mapstructure:"enabled"`
		CertPath string `mapstructure:"cert_path"`
		KeyPath  string `mapstructure:"key_path"`
	} `mapstructure:"tls"`
}

// DatabaseConfig holds the MongoDB connection details.
type DatabaseConfig struct {
	URL  string `mapstructure:"url"`
	Name string `mapstructure:"name"`
}

// RateLimiterConfig defines the rate limiting settings.
type RateLimiterConfig struct {
	Rate   int           `mapstructure:"rate"`   // Maximum requests per period.
	Burst  int           `mapstructure:"burst"`  // Maximum burst capacity.
	Period time.Duration `mapstructure:"period"` // Time period for rate limiting.
}

// RedisConfig holds the Redis connection details.
type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// SchedulerConfig holds the configuration for the subscription scheduler.
type SchedulerConfig struct {
	Interval      time.Duration `mapstructure:"interval"`        // Polling interval for reminders.
	ReminderDays  []int         `mapstructure:"reminder_days"`   // Days before renewal to send reminders.
	EnabledForEnv []string      `mapstructure:"enabled_for_env"` // Environments where the scheduler is enabled.
}

// QueueWorkerConfig holds the configuration for the queue worker.
type QueueWorkerConfig struct {
	Concurrency   int      `mapstructure:"concurrency"`     // Number of concurrent workers.
	QueueName     string   `mapstructure:"queue_name"`      // Name of the queue to process.
	EnabledForEnv []string `mapstructure:"enabled_for_env"` // Environments where the worker is enabled.
}

// OTelConfig holds the OpenTelemetry observability configuration.
type OTelConfig struct {
	Enabled        bool   `mapstructure:"enabled"`         // Enable OpenTelemetry instrumentation.
	ServiceName    string `mapstructure:"service_name"`    // Service name for traces and metrics.
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"` // OTLP gRPC endpoint for Jaeger.
}

// Config holds the complete application configuration.
type Config struct {
	Server      ServerConfig       `mapstructure:"server"`
	Database    DatabaseConfig     `mapstructure:"database"`
	JWT         services.JWTConfig `mapstructure:"jwt"`
	Redis       RedisConfig        `mapstructure:"redis"`
	Env         string             `mapstructure:"env"` // Current application environment (e.g., development, production).
	Scheduler   SchedulerConfig    `mapstructure:"scheduler"`
	QueueWorker QueueWorkerConfig  `mapstructure:"queue_worker"`
	Email       notifications.EmailConfig  `mapstructure:"email"`
	OTel        OTelConfig         `mapstructure:"otel"`

	RateLimiter struct {
		App RateLimiterConfig `mapstructure:"app"` // Application-level rate limiter settings.
	} `mapstructure:"rate_limiter"`
}
