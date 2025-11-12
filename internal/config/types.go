package config

import "time"

type Config struct {
	Server           ServerConfig    `yaml:"server"`
	RateLimits       []RateLimitRule `yaml:"rate_limits"`
	DefaultRateLimit *RateLimitRule  `yaml:"default_rate_limit"`
	CerebrasLimits   CerebrasLimits  `yaml:"cerebras_limits"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type RateLimitRule struct {
	Domain            string `yaml:"domain"`
	RequestsPerSecond int    `yaml:"requests_per_second"`
}

type CerebrasLimits struct {
	RPMLimit          int           `yaml:"rpm_limit"`
	TPMLimit          int           `yaml:"tpm_limit"`
	MaxQueueDepth     int           `yaml:"max_queue_depth"`
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	PriorityThreshold float64       `yaml:"priority_threshold"`
}

// Set default values for CerebrasLimits
func (c *CerebrasLimits) SetDefaults() {
	if c.RPMLimit == 0 {
		c.RPMLimit = 1000
	}
	if c.TPMLimit == 0 {
		c.TPMLimit = 1000000
	}
	if c.MaxQueueDepth == 0 {
		c.MaxQueueDepth = 100
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 10 * time.Minute
	}
	if c.PriorityThreshold == 0 {
		c.PriorityThreshold = 0.7
	}
}
