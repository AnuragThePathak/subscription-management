package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

func LoadConfig() (*Config, error) {
	// Set the config file name and type.
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	// Add config paths (current directory in this case)
	viper.AddConfigPath(".")

	// Set default values.
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.tls.enabled", false)
	viper.SetDefault("jwt.access_timeout", "1")
	viper.SetDefault("jwt.refresh_timeout", "72")
	viper.SetDefault("rate_limiter.requests_per_minute", 3*60)

	viper.SetDefault("scheduler.interval", "12h")
	viper.SetDefault("scheduler.reminder_days", [3]int{1, 3, 7})

	viper.SetDefault("queue_worker.concurrency", 2)
	viper.SetDefault("queue_worker.queue_name", "default")

	viper.SetDefault("email.smtp_port", 587)
	viper.SetDefault("email.from_name", "Subscription Management")

	// Read the YAML configuration file.
	if err := viper.ReadInConfig(); err != nil {
		// If the config file is missing, it will log an error,
		// but you might not want to stop execution if you're relying on env variables.
		slog.Debug("Config file not found, using defaults", slog.String("error", err.Error()))
	}

	// Enable environment variables to override config file settings.
	// Optionally set a prefix to avoid clashes.
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

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
	if c.Redis.URL == "" {
		missing = append(missing, "redis.url")
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
