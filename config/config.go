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
	RequestsPerMinute int `mapstructure:"requests_per_minute"`
}

// Complete application configuration
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	RateLimiter RateLimiterConfig `mapstructure:"rate_limiter"`
	Env         string            `mapstructure:"env"`
}
