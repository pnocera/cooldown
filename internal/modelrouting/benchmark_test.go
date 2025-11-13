package modelrouting

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

func BenchmarkModelRouting(b *testing.B) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4":    "https://api.openai.com/v1",
			"claude-3": "https://api.anthropic.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	json := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello!"}]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}

func BenchmarkModelRoutingDisabled(b *testing.B) {
	cfg := &config.ModelRoutingConfig{
		Enabled: false, // Disabled
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	json := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello!"}]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}

func BenchmarkModelRoutingNonJSON(b *testing.B) {
	cfg := &config.ModelRoutingConfig{
		Enabled: true,
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	body := `not json content`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}

func BenchmarkModelRoutingLargePayload(b *testing.B) {
	cfg := &config.ModelRoutingConfig{
		Enabled: true,
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	// Create large JSON payload (10KB)
	largeContent := strings.Repeat("x", 10000)
	json := `{"model": "gpt-4", "content": "` + largeContent + `"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}

func BenchmarkModelRoutingUnknownModel(b *testing.B) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	json := `{"model": "unknown-model", "messages": [{"role": "user", "content": "Hello!"}]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}
