package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestConfigParsing(t *testing.T) {
	yamlData := `
server:
  host: "localhost"
  port: 8080
rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10
default_rate_limit:
  requests_per_second: 1
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlData), &config)

	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if config.Server.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", config.Server.Host)
	}

	if len(config.RateLimits) != 1 {
		t.Errorf("Expected 1 rate limit rule, got %d", len(config.RateLimits))
	}
}

func TestLoadConfig_CerebrasLimits(t *testing.T) {
	configData := `
server:
  host: "localhost"
  port: 8080

cerebras_limits:
  rpm_limit: 1000
  tpm_limit: 1000000
  max_queue_depth: 100
  request_timeout: 10m
  priority_threshold: 0.7

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 100
`

	var config Config
	err := yaml.Unmarshal([]byte(configData), &config)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.CerebrasLimits.RPMLimit != 1000 {
		t.Errorf("Expected RPM limit 1000, got %d", config.CerebrasLimits.RPMLimit)
	}

	if config.CerebrasLimits.TPMLimit != 1000000 {
		t.Errorf("Expected TPM limit 1000000, got %d", config.CerebrasLimits.TPMLimit)
	}

	if config.CerebrasLimits.MaxQueueDepth != 100 {
		t.Errorf("Expected max queue depth 100, got %d", config.CerebrasLimits.MaxQueueDepth)
	}
}
