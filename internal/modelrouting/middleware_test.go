package modelrouting

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

func TestModelRoutingMiddleware(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4":    "https://api.openai.com/v1",
			"claude-3": "https://api.anthropic.com/v1",
		},
	}

	// Create a mock next handler that captures the request
	var capturedRequest *http.Request
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	t.Run("routes based on model field", func(t *testing.T) {
		body := bytes.NewBufferString(`{"model": "gpt-4", "messages": []}`)
		req := httptest.NewRequest("POST", "/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if capturedRequest == nil {
			t.Fatal("Request was not captured")
		}

		expectedHost := "api.openai.com"
		if capturedRequest.Host != expectedHost {
			t.Errorf("Expected host %s, got %s", expectedHost, capturedRequest.Host)
		}

		if capturedRequest.URL.Host != expectedHost {
			t.Errorf("Expected URL host %s, got %s", expectedHost, capturedRequest.URL.Host)
		}
	})

	t.Run("uses default target for unknown model", func(t *testing.T) {
		body := bytes.NewBufferString(`{"model": "unknown-model", "messages": []}`)
		req := httptest.NewRequest("POST", "/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if capturedRequest.Host != "api.openai.com" {
			t.Errorf("Expected default target host, got %s", capturedRequest.Host)
		}
	})

	t.Run("skips non-JSON requests", func(t *testing.T) {
		body := bytes.NewBufferString(`not json`)
		req := httptest.NewRequest("POST", "/chat/completions", body)
		req.Header.Set("Content-Type", "text/plain")

		originalHost := "original.example.com"
		req.Host = originalHost

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if capturedRequest.Host != originalHost {
			t.Errorf("Expected original host %s, got %s", originalHost, capturedRequest.Host)
		}
	})

	t.Run("disabled middleware passes through", func(t *testing.T) {
		cfgDisabled := &config.ModelRoutingConfig{
			Enabled:       false,
			DefaultTarget: "https://api.openai.com/v1",
			Models: map[string]string{
				"gpt-4": "https://api.openai.com/v1",
			},
		}
		middleware := NewModelRoutingMiddleware(cfgDisabled, nextHandler)

		body := bytes.NewBufferString(`{"model": "gpt-4"}`)
		req := httptest.NewRequest("POST", "/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")
		originalHost := "original.example.com"
		req.Host = originalHost

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if capturedRequest.Host != originalHost {
			t.Errorf("Expected original host when disabled, got %s", capturedRequest.Host)
		}
	})
}

func TestParseModelField(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	middleware := NewModelRoutingMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	t.Run("extracts model from valid JSON", func(t *testing.T) {
		json := `{"model": "gpt-4", "messages": []}`
		reader := strings.NewReader(json)

		target, err := middleware.ParseModelField(reader)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if target != "https://api.openai.com/v1" {
			t.Errorf("Expected OpenAI target, got '%s'", target)
		}
	})

	t.Run("returns empty for missing model field", func(t *testing.T) {
		json := `{"messages": []}`
		reader := strings.NewReader(json)

		target, err := middleware.ParseModelField(reader)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if target != "" {
			t.Errorf("Expected empty target for missing model, got %s", target)
		}
	})

	t.Run("handles empty JSON", func(t *testing.T) {
		json := `{}`
		reader := strings.NewReader(json)

		target, err := middleware.ParseModelField(reader)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if target != "" {
			t.Errorf("Expected empty target for empty JSON, got %s", target)
		}
	})
}