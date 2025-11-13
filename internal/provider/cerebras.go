package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

type CerebrasProvider struct {
	config       *config.ProviderConfig
	currentKey   int
	keyStats     map[string]*KeyStats
	mu           sync.Mutex
	httpClient   *http.Client
}

type KeyStats struct {
	Key                 string
	LastReset           time.Time
	RequestsUsed        int
	TokensUsed         int64
	LimitRequestsDay   int
	LimitTokensMinute  int
	RemainingRequests  int
	RemainingTokens    int
}

type CerebrasRequest struct {
	Model    string                 `json:"model"`
	Messages []CerebrasMessage      `json:"messages"`
	MaxTokens int                   `json:"max_tokens"`
	Stream   bool                   `json:"stream"`
	Tools    []CerebrasTool        `json:"tools,omitempty"`
}

type CerebrasMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CerebrasTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

func NewCerebrasProvider(config *config.ProviderConfig) *CerebrasProvider {
	provider := &CerebrasProvider{
		config:     config,
		currentKey: 0,
		keyStats:   make(map[string]*KeyStats),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	// Initialize key stats
	for _, keyConfig := range config.LoadBalancing.APIKeys {
		provider.keyStats[keyConfig.Key] = &KeyStats{
			Key:               keyConfig.Key,
			LastReset:         time.Now(),
			LimitRequestsDay:  1000, // default, will be updated from headers
			LimitTokensMinute: 10000, // default, will be updated from headers
		}
	}

	return provider
}

func (p *CerebrasProvider) Name() string {
	return "cerebras"
}

func (p *CerebrasProvider) GetAPIKey() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.config.LoadBalancing.Strategy == "round_robin" {
		key := p.config.LoadBalancing.APIKeys[p.currentKey].Key
		p.currentKey = (p.currentKey + 1) % len(p.config.LoadBalancing.APIKeys)
		return key
	}

	// Default to first key
	return p.config.LoadBalancing.APIKeys[0].Key
}

func (p *CerebrasProvider) CheckRateLimit() error {
	key := p.GetAPIKey()
	stats := p.keyStats[key]

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if we need to reset counters
	if time.Since(stats.LastReset) > 24*time.Hour {
		stats.RequestsUsed = 0
		stats.LastReset = time.Now()
	}

	// Check daily limit
	if stats.RemainingRequests < 100 {
		return fmt.Errorf("approaching daily request limit: %d remaining", stats.RemainingRequests)
	}

	// Check minute limit for tokens
	// This is simplified - in production, you'd want per-minute tracking
	if stats.RemainingTokens < 1000 {
		return fmt.Errorf("approaching minute token limit: %d remaining", stats.RemainingTokens)
	}

	return nil
}

func (p *CerebrasProvider) MakeRequest(model string, messages []interface{}, options map[string]interface{}) (*Response, error) {
	if err := p.CheckRateLimit(); err != nil {
		return nil, err
	}

	// Convert messages to Cerebras format
	cerebrasMessages := make([]CerebrasMessage, len(messages))
	for i, msg := range messages {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			cerebrasMessages[i] = CerebrasMessage{
				Role:    msgMap["role"].(string),
				Content: msgMap["content"].(string),
			}
		}
	}

	cerebrasReq := CerebrasRequest{
		Model:     model,
		Messages:  cerebrasMessages,
		MaxTokens: 1024, // default
		Stream:    false,
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		cerebrasReq.MaxTokens = maxTokens
	}

	reqBody, err := json.Marshal(cerebrasReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", p.config.Endpoint+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	apiKey := p.GetAPIKey()
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Update rate limit stats from headers
	p.updateRateLimitStats(apiKey, resp.Header)

	var cerebrasResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cerebrasResp); err != nil {
		return nil, err
	}

	content := ""
	if len(cerebrasResp.Choices) > 0 {
		content = cerebrasResp.Choices[0].Message.Content
	}

	// Convert headers to map
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &Response{
		Content: content,
		Model:   cerebrasResp.Model,
		Usage: map[string]interface{}{
			"prompt_tokens":     cerebrasResp.Usage.PromptTokens,
			"completion_tokens": cerebrasResp.Usage.CompletionTokens,
			"total_tokens":      cerebrasResp.Usage.TotalTokens,
		},
		Headers: headers,
	}, nil
}

func (p *CerebrasProvider) updateRateLimitStats(apiKey string, headers http.Header) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := p.keyStats[apiKey]

	// Parse rate limit headers
	if dayLimit := headers.Get("x-ratelimit-limit-requests-day"); dayLimit != "" {
		fmt.Sscanf(dayLimit, "%d", &stats.LimitRequestsDay)
	}
	if dayRemaining := headers.Get("x-ratelimit-remaining-requests-day"); dayRemaining != "" {
		fmt.Sscanf(dayRemaining, "%d", &stats.RemainingRequests)
	}
	if minuteLimit := headers.Get("x-ratelimit-limit-tokens-minute"); minuteLimit != "" {
		fmt.Sscanf(minuteLimit, "%d", &stats.LimitTokensMinute)
	}
	if minuteRemaining := headers.Get("x-ratelimit-remaining-tokens-minute"); minuteRemaining != "" {
		fmt.Sscanf(minuteRemaining, "%d", &stats.RemainingTokens)
	}

	// Increment usage counters
	stats.RequestsUsed++
}