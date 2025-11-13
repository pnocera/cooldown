package provider

import (
	"fmt"
	"sync"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

type Provider interface {
	Name() string
	MakeRequest(model string, messages []interface{}, options map[string]interface{}) (*Response, error)
	GetAPIKey() string
	CheckRateLimit() error
}

type Response struct {
	Content string                 `json:"content"`
	Model   string                 `json:"model"`
	Usage   map[string]interface{} `json:"usage"`
	Headers map[string]string      `json:"headers"`
}

type ProviderManager struct {
	config    *config.Config
	providers map[string]Provider
	mu        sync.RWMutex
}

func NewProviderManager(config *config.Config) *ProviderManager {
	pm := &ProviderManager{
		config:    config,
		providers: make(map[string]Provider),
	}

	// Initialize providers
	for _, providerConfig := range config.Providers {
		switch providerConfig.Name {
		case "cerebras":
			pm.providers[providerConfig.Name] = NewCerebrasProvider(&providerConfig)
		case "zhipu":
			pm.providers[providerConfig.Name] = NewZhipuProvider(&providerConfig)
		}
	}

	return pm
}

func (pm *ProviderManager) GetProviderForModel(model string) (Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, providerConfig := range pm.config.Providers {
		for _, providerModel := range providerConfig.Models {
			if providerModel == model {
				if provider, exists := pm.providers[providerConfig.Name]; exists {
					return provider, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no provider found for model: %s", model)
}
