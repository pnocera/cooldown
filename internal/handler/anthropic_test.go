package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestAnthropicEndpointBasics(t *testing.T) {
	handler := NewAnthropicHandler(&config.Config{})

	req := httptest.NewRequest("POST", "/anthropic/v1/messages", strings.NewReader(`{
        "model": "claude-3-5-sonnet-20241022",
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": "Hello"}]
    }`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}