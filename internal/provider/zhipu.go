package provider

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
)

type ZhipuProvider struct {
	config *config.ProviderConfig
}

func NewZhipuProvider(config *config.ProviderConfig) *ZhipuProvider {
	return &ZhipuProvider{config: config}
}

func (p *ZhipuProvider) Name() string {
	return "zhipu"
}

func (p *ZhipuProvider) GetAPIKey() string {
	return p.config.APIKey
}

func (p *ZhipuProvider) CheckRateLimit() error {
	// Zhipu has fixed rate limits, implement basic checking
	return nil
}

func (p *ZhipuProvider) MakeRequest(model string, messages []interface{}, options map[string]interface{}) (*Response, error) {
	// TODO: Implement Zhipu API integration
	return &Response{
		Content: "Zhipu response placeholder",
		Model:   model,
	}, nil
}
