package router

import (
	"net/http/httptest"
	"testing"
)

func TestCerebrasRouter_IsCerebrasRequest(t *testing.T) {
	router := NewCerebrasRouter(nil)

	// Test with Cerebras API host
	req := httptest.NewRequest("POST", "https://api.cerebras.ai/v1/chat/completions", nil)
	req.Host = "api.cerebras.ai"

	if !router.IsCerebrasRequest(req) {
		t.Error("Expected request to api.cerebras.ai to be detected as Cerebras request")
	}

	// Test with non-Cerebras host
	req2 := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req2.Host = "api.openai.com"

	if router.IsCerebrasRequest(req2) {
		t.Error("Expected request to api.openai.com to NOT be detected as Cerebras request")
	}
}
