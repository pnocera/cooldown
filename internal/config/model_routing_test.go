package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestModelRoutingConfigLoading(t *testing.T) {
	yamlData := `
server:
  host: "localhost"
  port: 8080
model_routing:
  enabled: true
  default_target: "https://api.openai.com/v1"
  models:
    "gpt-4": "https://api.openai.com/v1"
    "claude-3": "https://api.anthropic.com/v1"
`

	var cfg Config
	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.ModelRouting == nil {
		t.Fatal("ModelRouting should not be nil")
	}

	if !cfg.ModelRouting.Enabled {
		t.Error("Model routing should be enabled")
	}

	if cfg.ModelRouting.DefaultTarget != "https://api.openai.com/v1" {
		t.Errorf("Expected default target https://api.openai.com/v1, got %s", cfg.ModelRouting.DefaultTarget)
	}

	if cfg.ModelRouting.Models["gpt-4"] != "https://api.openai.com/v1" {
		t.Errorf("Expected gpt-4 to map to OpenAI, got %s", cfg.ModelRouting.Models["gpt-4"])
	}

	if cfg.ModelRouting.Models["claude-3"] != "https://api.anthropic.com/v1" {
		t.Errorf("Expected claude-3 to map to Anthropic, got %s", cfg.ModelRouting.Models["claude-3"])
	}
}

func TestModelRoutingDisabledByDefault(t *testing.T) {
	yamlData := `
server:
  host: "localhost"
  port: 8080
`

	var cfg Config
	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.ModelRouting != nil && cfg.ModelRouting.Enabled {
		t.Error("Model routing should be disabled by default")
	}
}
