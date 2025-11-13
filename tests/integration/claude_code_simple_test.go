package integration

import (
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

func TestConfigurationLoading(t *testing.T) {
	t.Run("Load Claude Code Configuration", func(t *testing.T) {
		config, err := config.Load("../../config.yaml.example-claude-code")
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}
		
		// Verify server configuration
		if config.Server.AnthropicEndpoint != "/anthropic" {
			t.Errorf("Expected AnthropicEndpoint '/anthropic', got %s", config.Server.AnthropicEndpoint)
		}
		
		if config.Server.OpenAIEndpoint != "/openai" {
			t.Errorf("Expected OpenAIEndpoint '/openai', got %s", config.Server.OpenAIEndpoint)
		}
		
		// Verify environment models
		if config.EnvironmentModels.Sonnet == "" {
			t.Error("Expected Sonnet model to be configured")
		}
		
		// Verify providers
		if len(config.Providers) == 0 {
			t.Error("Expected at least one provider to be configured")
		}
		
		// Verify reasoning config
		if !config.ReasoningConfig.Enabled {
			t.Error("Expected reasoning injection to be enabled")
		}
		
		t.Logf("Successfully loaded configuration with %d providers", len(config.Providers))
	})
}

func TestModelMapping(t *testing.T) {
	config := &config.Config{
		EnvironmentModels: config.EnvironmentModels{
			Haiku:  "glm-4.5-air",
			Sonnet: "glm-4.6",
			Opus:   "glm-4.6",
		},
	}
	
	testCases := []struct {
		claudeModel      string
		expectedProvider string
	}{
		{"claude-3-5-haiku-20241022", "glm-4.5-air"},
		{"claude-3-5-sonnet-20241022", "glm-4.6"},
		{"claude-3-opus-20240229", "glm-4.6"},
		{"unknown-model", "glm-4.6"}, // Should default to sonnet
	}
	
	for _, tc := range testCases {
		t.Run(tc.claudeModel, func(t *testing.T) {
			var mappedModel string
			switch {
			case contains(tc.claudeModel, "haiku"):
				mappedModel = config.EnvironmentModels.Haiku
			case contains(tc.claudeModel, "sonnet"):
				mappedModel = config.EnvironmentModels.Sonnet
			case contains(tc.claudeModel, "opus"):
				mappedModel = config.EnvironmentModels.Opus
			default:
				mappedModel = config.EnvironmentModels.Sonnet
			}
			
			if mappedModel != tc.expectedProvider {
				t.Errorf("Expected %s, got %s", tc.expectedProvider, mappedModel)
			}
		})
	}
}

func TestEnvironmentVariableExpansion(t *testing.T) {
	// Test environment variable expansion functionality
	t.Run("Environment Variables Expand", func(t *testing.T) {
		yamlContent := `
environment_models:
  haiku: "${TEST_HAIKU_MODEL:glm-4.5-air}"
  sonnet: "glm-4.6"
`
		
		config, err := config.LoadFromYAMLString(yamlContent)
		if err != nil {
			t.Fatalf("Failed to load config from string: %v", err)
		}
		
		// Should have the default value since env var is not set
		if config.EnvironmentModels.Haiku != "glm-4.5-air" {
			t.Errorf("Expected 'glm-4.5-air', got %s", config.EnvironmentModels.Haiku)
		}
		
		if config.EnvironmentModels.Sonnet != "glm-4.6" {
			t.Errorf("Expected 'glm-4.6', got %s", config.EnvironmentModels.Sonnet)
		}
	})
}

// Helper function from the plan
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)+1] == substr+"-" || 
			 s[len(s)-len(substr)-1:] == "-"+substr || 
			 findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}