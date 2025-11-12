package config

type Config struct {
	Server           ServerConfig      `yaml:"server"`
	RateLimits       []RateLimitRule   `yaml:"rate_limits"`
	DefaultRateLimit *RateLimitRule    `yaml:"default_rate_limit"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type RateLimitRule struct {
	Domain           string `yaml:"domain"`
	RequestsPerSecond int    `yaml:"requests_per_second"`
}