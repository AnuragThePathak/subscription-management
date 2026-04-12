package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

// LoadConfig loads the application configuration from a YAML file or environment variables.
func LoadConfig(ctx context.Context) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Set default values for configuration.
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.request_timeout", "10s")
	viper.SetDefault("server.tls.enabled", false)
	viper.SetDefault("jwt.access_timeout", "1")
	viper.SetDefault("jwt.refresh_timeout", "72")
	viper.SetDefault("rate_limiter.requests_per_minute", 3*60)
	viper.SetDefault("scheduler.interval", "12h")
	viper.SetDefault("scheduler.reminder_days", [3]int{1, 3, 7})
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
	if err := viper.ReadInConfig(); err != nil {
		slog.WarnContext(ctx, "Config file not found, using defaults", slog.Any("error", err))
	}

	// Enable environment variables to override config file settings.
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal configuration", slog.Any("error", err))
		return nil, err
	}
	if err := config.Validate(); err != nil {
		slog.ErrorContext(ctx, "Configuration validation failed", slog.Any("error", err))
		return nil, err
	}
	slog.InfoContext(ctx, "Configuration loaded successfully")
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

	if c.Database.URL == "" {
		missing = append(missing, "database.url")
	}
	if c.Database.Name == "" {
		missing = append(missing, "database.name")
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
	if c.Redis.URL == "" {
		missing = append(missing, "redis.url")
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
		return fmt.Errorf("missing required config fields: %v", missing)
	}

	return nil
}
