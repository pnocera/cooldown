package config

import "time"

type EnvironmentModels struct {
	Haiku  string `yaml:"haiku"`
	Sonnet string `yaml:"sonnet"`
	Opus   string `yaml:"opus"`
}

type ProviderConfig struct {
	Name          string                   `yaml:"name"`
	Endpoint      string                   `yaml:"endpoint"`
	Models        []string                 `yaml:"models"`
	LoadBalancing *LoadBalancingConfig     `yaml:"load_balancing,omitempty"`
	APIKey        string                   `yaml:"api_key,omitempty"`
	RateLimiting  *ProviderRateLimitConfig `yaml:"rate_limiting,omitempty"`
}

type LoadBalancingConfig struct {
	Strategy string         `yaml:"strategy"` // round_robin, least_used, weighted_random
	APIKeys  []APIKeyConfig `yaml:"api_keys"`
}

type APIKeyConfig struct {
	Key                  string `yaml:"key"`
	Weight               int    `yaml:"weight"`
	MaxRequestsPerMinute int    `yaml:"max_requests_per_minute"`
}

type ProviderRateLimitConfig struct {
	Type              string  `yaml:"type"` // per_key_cerebras_headers, fixed_rpm, tokens_per_minute
	SafetyMargin      float64 `yaml:"safety_margin,omitempty"`
	BackoffThreshold  int     `yaml:"backoff_threshold,omitempty"`
	RequestsPerMinute int     `yaml:"requests_per_minute,omitempty"`
	TokensPerMinute   int     `yaml:"tokens_per_minute,omitempty"`
}

type ReasoningConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Models         []string `yaml:"models"`
	PromptTemplate string   `yaml:"prompt_template"`
}

type MonitoringConfig struct {
	MetricsEnabled     bool   `yaml:"metrics_enabled"`
	HealthEndpoint     string `yaml:"health_endpoint"`
	PrometheusEndpoint string `yaml:"prometheus_endpoint"`
	LogLevel           string `yaml:"log_level"`
}

type Config struct {
	Server            ServerConfig        `yaml:"server"`
	EnvironmentModels EnvironmentModels   `yaml:"environment_models"`
	Providers         []ProviderConfig    `yaml:"providers"`
	ReasoningConfig   ReasoningConfig     `yaml:"reasoning_injection"`
	RateLimits        []RateLimitRule     `yaml:"rate_limits"`
	DefaultRateLimit  *RateLimitRule      `yaml:"default_rate_limit"`
	CerebrasLimits    CerebrasLimits      `yaml:"cerebras_limits"`
	ModelRouting      *ModelRoutingConfig `yaml:"model_routing"`
	Monitoring        MonitoringConfig    `yaml:"monitoring,omitempty"`
}

type ServerConfig struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	BindAddress       string `yaml:"bind_address"`
	APIKeyRequired    bool   `yaml:"api_key_required"`
	AnthropicEndpoint string `yaml:"anthropic_endpoint"`
	OpenAIEndpoint    string `yaml:"openai_endpoint"`
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
