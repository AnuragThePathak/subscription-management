package config

import (
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/notifications"
	"github.com/anuragthepathak/subscription-management/internal/observability"
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
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Name       string `mapstructure:"name"`
	AuthSource string `mapstructure:"auth_source"`
}

// RateLimiterConfig defines the rate limiting settings.
type RateLimiterConfig struct {
	Rate   int           `mapstructure:"rate"`   // Maximum requests per period.
	Burst  int           `mapstructure:"burst"`  // Maximum burst capacity.
	Period time.Duration `mapstructure:"period"` // Time period for rate limiting.
}

// RedisConfig holds the Redis connection details.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// AsynqConfig holds the configuration for the Asynq queue.
type AsynqConfig struct {
	QueueName string `mapstructure:"queue_name"`
}

// SchedulerConfig holds the configuration for the subscription scheduler.
type SchedulerConfig struct {
	Name          string        `mapstructure:"name"`
	Interval      time.Duration `mapstructure:"interval"`        // Polling interval for reminders.
	ReminderDays  []int         `mapstructure:"reminder_days"`   // Days before renewal to send reminders.
	StartupDelay  time.Duration `mapstructure:"startup_delay"`   // Delay before the first poll on startup.
	EnabledForEnv []string      `mapstructure:"enabled_for_env"` // Environments where the scheduler is enabled.
}

// QueueWorkerConfig holds the configuration for the queue worker.
type QueueWorkerConfig struct {
	Name          string   `mapstructure:"name"`
	Concurrency   int      `mapstructure:"concurrency"`     // Number of concurrent workers.
	EnabledForEnv []string `mapstructure:"enabled_for_env"` // Environments where the worker is enabled.
}

// Config holds the complete application configuration.
type Config struct {
	Server      ServerConfig              `mapstructure:"server"`
	Database    DatabaseConfig            `mapstructure:"database"`
	JWT         services.JWTConfig        `mapstructure:"jwt"`
	Redis       RedisConfig               `mapstructure:"redis"`
	Asynq       AsynqConfig               `mapstructure:"asynq"`
	Env         string                    `mapstructure:"env"` // Current application environment (e.g., development, production).
	Scheduler   SchedulerConfig           `mapstructure:"scheduler"`
	QueueWorker QueueWorkerConfig         `mapstructure:"queue_worker"`
	Email       notifications.EmailConfig `mapstructure:"email"`
	OTel        observability.Config      `mapstructure:"otel"`

	RateLimiter struct {
		App RateLimiterConfig `mapstructure:"app"` // Application-level rate limiter settings.
	} `mapstructure:"rate_limiter"`
}
