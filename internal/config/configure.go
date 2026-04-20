package config

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/spf13/viper"
)

// LoadConfig loads the application configuration from a YAML file or environment variables.
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Set default values for configuration.
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.request_timeout", "10s")
	viper.SetDefault("server.tls.enabled", false)
	viper.SetDefault("database.auth_source", "admin")
	viper.SetDefault("database.port", 27017)
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.access_timeout", "1")
	viper.SetDefault("jwt.refresh_timeout", "72")
	viper.SetDefault("rate_limiter.requests_per_minute", 3*60)
	viper.SetDefault("scheduler.interval", "12h")
	viper.SetDefault("scheduler.reminder_days", [3]int{1, 3, 7})
	viper.SetDefault("scheduler.startup_delay", "15m")
	viper.SetDefault("scheduler.enabled_for_env", []string{"development", "staging", "production"})
	viper.SetDefault("queue_worker.concurrency", 2)
	viper.SetDefault("queue_worker.queue_name", "default")
	viper.SetDefault("queue_worker.enabled_for_env", []string{"development", "staging", "production"})
	viper.SetDefault("otel.enabled", false)
	viper.SetDefault("otel.service_name", "subscription-management")
	viper.SetDefault("otel.jaeger_endpoint", "localhost:4317")
	viper.SetDefault("email.smtp_port", 587)
	viper.SetDefault("email.from_name", "Subscription Management")

	// Read the YAML configuration file.
	if err := viper.ReadInConfig(); err != nil &&
		!errors.As(err, &viper.ConfigFileNotFoundError{}) {
		return nil, fmt.Errorf("config file found but failed to parse: %w", err)
	}

	// Enable environment variables to override config file settings.
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	slog.Info("Configuration loaded successfully",
		logattr.Env(config.Env),
		logattr.ConfigFile(viper.ConfigFileUsed()),
	)
	return &config, nil
}

// Validate checks for missing or invalid configuration fields.
func (c *Config) Validate() error {
	var missing []string

	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertPath == "" {
			missing = append(missing, "server.tls.cert_path")
		}
		if c.Server.TLS.KeyPath == "" {
			missing = append(missing, "server.tls.key_path")
		}
	}

	// Database configuration validation
	if c.Database.Host == "" {
		missing = append(missing, "database.host")
	}
	if c.Database.Username == "" {
		missing = append(missing, "database.username")
	}
	if c.Database.Password == "" {
		missing = append(missing, "database.password")
	}
	if c.Database.Name == "" {
		missing = append(missing, "database.name")
	}
	if c.Database.AuthSource == "" {
		missing = append(missing, "database.auth_source")
	}
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		missing = append(missing, "database.port (must be between 1 and 65535)")
	}

	// Redis configuration validation
	if c.Redis.Host == "" {
		missing = append(missing, "redis.host")
	}
	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		missing = append(missing, "redis.port (must be between 1 and 65535)")
	}
	if c.Redis.DB < 0 {
		missing = append(missing, "redis.db (must be 0 or greater)")
	}
	if c.JWT.AccessSecret == "" {
		missing = append(missing, "jwt.access_secret")
	}
	if c.JWT.RefreshSecret == "" {
		missing = append(missing, "jwt.refresh_secret")
	}
	if c.JWT.Issuer == "" {
		missing = append(missing, "jwt.issuer")
	}
	if c.Redis.Host == "" {
		missing = append(missing, "redis.host")
	}
	if c.RateLimiter.App.Rate == 0 {
		missing = append(missing, "rate_limiter.app.rate")
	}
	if c.Email.SMTPHost == "" {
		missing = append(missing, "email.smtp_host")
	}
	if c.Email.FromEmail == "" {
		missing = append(missing, "email.from_email")
	}
	if c.Email.SMTPUsername == "" {
		missing = append(missing, "email.smtp_username")
	}
	if c.Email.SMTPPassword == "" {
		missing = append(missing, "email.smtp_password")
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			"%d missing required config fields: %v",
			len(missing),
			missing,
		)
	}

	return nil
}
