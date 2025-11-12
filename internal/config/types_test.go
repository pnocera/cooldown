package config

import (
	"testing"
	"gopkg.in/yaml.v3"
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