package config

import "time"

type Config struct {
	Server           ServerConfig        `yaml:"server"`
	RateLimits       []RateLimitRule     `yaml:"rate_limits"`
	DefaultRateLimit *RateLimitRule      `yaml:"default_rate_limit"`
	CerebrasLimits   CerebrasLimits      `yaml:"cerebras_limits"`
	ModelRouting     *ModelRoutingConfig `yaml:"model_routing"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type RateLimitRule struct {
	Domain            string `yaml:"domain"`
	RequestsPerSecond int    `yaml:"requests_per_second"`
}

type CerebrasRateLimitConfig struct {
	UseHeaders     bool          `yaml:"use_headers"`
	HeaderFallback bool          `yaml:"header_fallback"`
	HeaderTimeout  time.Duration `yaml:"header_timeout"`
	ResetBuffer    time.Duration `yaml:"reset_buffer"`
}

type CerebrasLimits struct {
	RateLimits        CerebrasRateLimitConfig `yaml:"rate_limits"`
	RPMLimit          int                     `yaml:"rpm_limit"`
	TPMLimit          int                     `yaml:"tpm_limit"`
	MaxQueueDepth     int                     `yaml:"max_queue_depth"`
	RequestTimeout    time.Duration           `yaml:"request_timeout"`
	PriorityThreshold float64                 `yaml:"priority_threshold"`
}

type ModelRoutingConfig struct {
	Enabled       bool              `yaml:"enabled"`
	DefaultTarget string            `yaml:"default_target"`
	Models        map[string]string `yaml:"models"`
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

	// Set defaults for rate limit config
	// Note: UseHeaders defaults to false (disabled by default)
	// Note: HeaderFallback defaults to true (enabled by default)
	c.RateLimits.HeaderFallback = true
	if c.RateLimits.HeaderTimeout == 0 {
		c.RateLimits.HeaderTimeout = 5 * time.Second
	}
	if c.RateLimits.ResetBuffer == 0 {
		c.RateLimits.ResetBuffer = 100 * time.Millisecond
	}
}
