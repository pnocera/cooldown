package reasoning

import (
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestReasoningInjectorInjectsForGLMModels(t *testing.T) {
	config := &config.Config{
		ReasoningConfig: config.ReasoningConfig{
			Enabled:        true,
			Models:         []string{"glm-4.6", "glm-4.5-air"},
			PromptTemplate: "You are an expert reasoning model. Always think step by step.",
		},
	}

	injector := NewReasoningInjector(config)

	messages := []map[string]interface{}{
		{"role": "user", "content": "What is 2+2?"},
	}

	result := injector.InjectIfRequired("glm-4.6", messages)

	assert.Len(t, result, 2)
	assert.Contains(t, result[0]["content"], "reasoning model")
	assert.Equal(t, "user", result[1]["role"])
}
