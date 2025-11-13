package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/model"
	"github.com/cooldownp/cooldown-proxy/internal/provider"
	"github.com/cooldownp/cooldown-proxy/internal/reasoning"
)

type AnthropicHandler struct {
	config          *config.Config
	router          *Router
	modelRouter     *model.ModelRouter
	providerManager *provider.ProviderManager
	reasonInjector  *reasoning.ReasoningInjector
}

type Router struct {
	// We'll implement a simple router for now
	routes map[string]*url.URL
}

type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	Tools     []AnthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type AnthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []AnthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason,omitempty"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func NewRouter(config *config.Config) *Router {
	return &Router{
		routes: make(map[string]*url.URL),
	}
}

func NewAnthropicHandler(config *config.Config) *AnthropicHandler {
	return &AnthropicHandler{
		config:          config,
		router:          NewRouter(config),
		modelRouter:     model.NewModelRouter(config),
		providerManager: provider.NewProviderManager(config),
		reasonInjector:  reasoning.NewReasoningInjector(config),
	}
}

func (h *AnthropicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var anthropicReq AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Map Claude model to provider model
	providerModel := h.modelRouter.MapModel(anthropicReq.Model)

	// Convert Anthropic messages to provider format
	providerMessages := make([]map[string]interface{}, len(anthropicReq.Messages))
	for i, msg := range anthropicReq.Messages {
		providerMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	// Inject reasoning if required
	providerMessages = h.reasonInjector.InjectIfRequired(providerModel, providerMessages)

	// Get provider for the model
	provider, err := h.providerManager.GetProviderForModel(providerModel)
	if err != nil {
		http.Error(w, fmt.Sprintf("No provider for model %s: %v", providerModel, err), http.StatusBadRequest)
		return
	}

	// Make request to provider
	options := map[string]interface{}{
		"max_tokens": anthropicReq.MaxTokens,
		"stream":     anthropicReq.Stream,
	}

	if len(anthropicReq.Tools) > 0 {
		options["tools"] = anthropicReq.Tools
	}

	providerResp, err := provider.MakeRequest(providerModel, convertToInterfaceSlice(providerMessages), options)
	if err != nil {
		http.Error(w, fmt.Sprintf("Provider error: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract reasoning if present
	reasoning, content := h.reasonInjector.ExtractReasoningFromResponse(providerResp.Content)

	// Build Anthropic response
	response := AnthropicResponse{
		ID:      fmt.Sprintf("msg_%d", time.Now().Unix()),
		Type:    "message",
		Role:    "assistant",
		Model:   anthropicReq.Model,
		Content: []AnthropicContent{},
	}

	// Add thinking block if reasoning was extracted
	if reasoning != "" {
		response.Content = append(response.Content, AnthropicContent{
			Type: "thinking",
			Text: reasoning,
		})
	}

	// Add main content
	response.Content = append(response.Content, AnthropicContent{
		Type: "text",
		Text: content,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func convertToInterfaceSlice(messages []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = msg
	}
	return result
}
