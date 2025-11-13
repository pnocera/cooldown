package reasoning

import (
	"strings"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

type ReasoningInjector struct {
	config *config.Config
}

func NewReasoningInjector(config *config.Config) *ReasoningInjector {
	return &ReasoningInjector{config: config}
}

func (r *ReasoningInjector) InjectIfRequired(model string, messages []map[string]interface{}) []map[string]interface{} {
	if !r.config.ReasoningConfig.Enabled {
		return messages
	}

	// Check if model requires reasoning injection
	requiresReasoning := false
	for _, reasoningModel := range r.config.ReasoningConfig.Models {
		if strings.Contains(model, reasoningModel) {
			requiresReasoning = true
			break
		}
	}

	if !requiresReasoning {
		return messages
	}

	// Check if reasoning is already present in conversation
	for _, msg := range messages {
		if content, ok := msg["content"].(string); ok {
			if strings.Contains(content, "reasoning") || strings.Contains(content, "thinking") {
				return messages // reasoning already present
			}
		}
	}

	// Inject reasoning prompt at the beginning
	reasoningMessage := map[string]interface{}{
		"role":    "system",
		"content": r.config.ReasoningConfig.PromptTemplate,
	}

	// Insert reasoning message as first message
	newMessages := make([]map[string]interface{}, len(messages)+1)
	newMessages[0] = reasoningMessage
	copy(newMessages[1:], messages)

	return newMessages
}

func (r *ReasoningInjector) ExtractReasoningFromResponse(response string) (string, string) {
	// Look for reasoning blocks in various formats
	if strings.Contains(response, "<reasoning_content>") {
		start := strings.Index(response, "<reasoning_content>")
		end := strings.Index(response, "</reasoning_content>")
		if start != -1 && end != -1 {
			reasoning := response[start+len("<reasoning_content>") : end]
			content := response[:start] + response[end+len("</reasoning_content>"):]
			return reasoning, strings.TrimSpace(content)
		}
	}

	if strings.Contains(response, "```thinking") {
		start := strings.Index(response, "```thinking")
		end := strings.Index(response[start:], "```")
		if start != -1 && end != -1 {
			reasoning := response[start+11 : start+end]
			content := response[:start] + response[start+end+3:]
			return reasoning, strings.TrimSpace(content)
		}
	}

	return "", response
}