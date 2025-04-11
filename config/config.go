package config

import "time"

// Server configuration
type ServerConfig struct {
	Port int `mapstructure:"port"`
	TLS  struct {
		Enabled  bool   `mapstructure:"enabled"`
		CertPath string `mapstructure:"cert_path"`
		KeyPath  string `mapstructure:"key_path"`
	} `mapstructure:"tls"`
}

// Database configuration
type DatabaseConfig struct {
	URL  string `mapstructure:"url"`
	Name string `mapstructure:"name"`
}

// JWT configuration
type JWTConfig struct {
	AccessSecret       string `mapstructure:"access_secret"`
	RefreshSecret      string `mapstructure:"refresh_secret"`
	AccessExpiryHours  int    `mapstructure:"access_timeout"`
	RefreshExpiryHours int    `mapstructure:"refresh_timeout"`
	Issuer             string `mapstructure:"issuer"`
}

// Rate limiter configuration
type RateLimiterConfig struct {
	Rate   int           `mapstructure:"rate"`
	Burst  int           `mapstructure:"burst"`
	Period time.Duration `mapstructure:"period"`
}

// Redis configuration
type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// SchedulerConfig represents the configuration for the scheduler
type SchedulerConfig struct {
	Interval      time.Duration `mapstructure:"interval"`
	ReminderDays  []int         `mapstructure:"reminder_days"`
	EnabledForEnv []string      `mapstructure:"enabled_for_env"`
}

// WorkerConfig represents the configuration for the worker
type QueueWorkerConfig struct {
	Concurrency   int      `mapstructure:"concurrency"`
	QueueName     string   `mapstructure:"queue_name"`
	EnabledForEnv []string `mapstructure:"enabled_for_env"`
}

// Complete application configuration
type Config struct {
	Server      ServerConfig   `mapstructure:"server"`
	Database    DatabaseConfig `mapstructure:"database"`
	JWT         JWTConfig      `mapstructure:"jwt"`
	Redis       RedisConfig    `mapstructure:"redis"`
	Env         string         `mapstructure:"env"`
	RateLimiter struct {
		App RateLimiterConfig `mapstructure:"app"`
	} `mapstructure:"rate_limiter"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	QueueWorker    QueueWorkerConfig `mapstructure:"queue_worker"`
}
