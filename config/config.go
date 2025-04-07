package config

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
	Rate     int    `mapstructure:"rate"`
	Burst    int    `mapstructure:"burst"`
	Duration int    `mapstructure:"duration"`
	Unit     string `mapstructure:"unit"`
}

// Redis configuration
type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Complete application configuration
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Env         string            `mapstructure:"env"`
	RateLimiter struct {
		App RateLimiterConfig `mapstructure:"app"`
	} `mapstructure:"rate_limiter"`
}
