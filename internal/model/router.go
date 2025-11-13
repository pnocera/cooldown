package model

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
)

type ModelRouter struct {
	config *config.Config
}

func NewModelRouter(config *config.Config) *ModelRouter {
	return &ModelRouter{config: config}
}

func (r *ModelRouter) MapModel(claudeModel string) string {
	switch {
	case contains(claudeModel, "haiku"):
		return r.config.EnvironmentModels.Haiku
	case contains(claudeModel, "sonnet"):
		return r.config.EnvironmentModels.Sonnet
	case contains(claudeModel, "opus"):
		return r.config.EnvironmentModels.Opus
	default:
		return r.config.EnvironmentModels.Sonnet // default fallback
	}
}

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

func (r *ModelRouter) GetProviderForModel(model string) *config.ProviderConfig {
	for _, provider := range r.config.Providers {
		for _, providerModel := range provider.Models {
			if providerModel == model {
				return &provider
			}
		}
	}
	return nil
}