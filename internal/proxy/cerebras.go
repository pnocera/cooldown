package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/circuitbreaker"
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
)

type CerebrasProxyHandler struct {
	Limiter        *ratelimit.CerebrasLimiter
	Estimator      *token.TokenEstimator
	Config         *config.CerebrasLimits
	proxy          *httputil.ReverseProxy
	cerebrasHosts  []string
	circuitBreaker *circuitbreaker.CircuitBreaker
}

func NewCerebrasProxyHandler(
	limiter *ratelimit.CerebrasLimiter,
	estimator *token.TokenEstimator,
	config *config.CerebrasLimits,
) *CerebrasProxyHandler {
	// Create reverse proxy director
	director := func(req *http.Request) {
		// Set target to Cerebras API
		req.URL.Scheme = "https"
		req.URL.Host = "api.cerebras.ai"
		req.Host = "api.cerebras.ai"
	}

	// Initialize circuit breaker
	circuitBreaker := circuitbreaker.NewCircuitBreaker(circuitbreaker.Config{
		Name:             "cerebras-api",
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 3,
		OnStateChange: func(name string, from, to circuitbreaker.State) {
			// Log circuit breaker state changes
			fmt.Printf("Circuit breaker '%s' state changed: %s -> %s\n", name, from.String(), to.String())
		},
	})

	return &CerebrasProxyHandler{
		Limiter:   limiter,
		Estimator: estimator,
		Config:    config,
		cerebrasHosts: []string{
			"api.cerebras.ai",
			"inference.cerebras.ai",
		},
		circuitBreaker: circuitBreaker,
		proxy: &httputil.ReverseProxy{
			Director: director,
			ModifyResponse: func(resp *http.Response) error {
				// Add rate limit headers
				resp.Header.Set("X-RateLimit-Limit-RPM", strconv.Itoa(config.RPMLimit))
				resp.Header.Set("X-RateLimit-Limit-TPM", strconv.Itoa(config.TPMLimit))
				resp.Header.Set("X-RateLimit-Queue-Length", strconv.Itoa(limiter.QueueLength()))

				// Add circuit breaker state headers
				stats := circuitBreaker.Stats()
				resp.Header.Set("X-CircuitBreaker-State", stats.State.String())
				resp.Header.Set("X-CircuitBreaker-Failures", strconv.Itoa(stats.Failures))

				return nil
			},
		},
	}
}

func (h *CerebrasProxyHandler) IsCerebrasRequest(req *http.Request) bool {
	host := req.Host
	// Remove port if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	for _, cerebrasHost := range h.cerebrasHosts {
		if host == cerebrasHost {
			return true
		}
	}

	return false
}

func (h *CerebrasProxyHandler) EstimateTokens(req *http.Request) (int, error) {
	if req.Body == nil {
		return 0, fmt.Errorf("request body is nil")
	}

	// Read body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read request body: %w", err)
	}

	// Restore body for later use
	req.Body = io.NopCloser(bytes.NewReader(body))

	// Parse JSON to extract content
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract model and messages for chat completions
	model, _ := requestData["model"].(string)
	messagesInterface, messagesExist := requestData["messages"].([]interface{})
	maxTokensInterface, maxTokensExists := requestData["max_tokens"]

	// Create token request
	tokenReq := &token.TokenRequest{
		Model: model,
	}

	// Extract input tokens from messages
	if messagesExist {
		var totalWords int
		for _, msgInterface := range messagesInterface {
			if msg, ok := msgInterface.(map[string]interface{}); ok {
				if content, contentOk := msg["content"].(string); contentOk {
					// Simple word count for token estimation
					words := len(bytes.Split([]byte(content), []byte(" ")))
					totalWords += words
				}
			}
		}
		// Estimate input tokens (rough approximation: 1 token ≈ 0.75 words)
		tokenReq.InputTokens = int(float64(totalWords) / 0.75)
	}

	// Extract output tokens from max_tokens
	if maxTokensExists {
		if maxTokens, ok := maxTokensInterface.(float64); ok {
			tokenReq.OutputTokens = int(maxTokens)
		}
	}

	// If no max_tokens specified, estimate conservatively (50% of input)
	if tokenReq.OutputTokens == 0 && tokenReq.InputTokens > 0 {
		tokenReq.OutputTokens = tokenReq.InputTokens / 2
	}

	return h.Estimator.EstimateTokens(tokenReq), nil
}

func (h *CerebrasProxyHandler) CheckRateLimit(req *http.Request, tokens int) time.Duration {
	requestID := h.generateRequestID(req)
	return h.Limiter.CheckRequestWithQueue(requestID, tokens)
}

func (h *CerebrasProxyHandler) generateRequestID(req *http.Request) string {
	return fmt.Sprintf("cerebras-%d-%s", time.Now().UnixNano(), req.Host)
}

func (h *CerebrasProxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Only handle Cerebras requests
	if !h.IsCerebrasRequest(req) {
		http.Error(w, "Not a Cerebras request", http.StatusBadRequest)
		return
	}

	// Estimate tokens for rate limiting
	tokens, err := h.EstimateTokens(req)
	if err != nil {
		// Log error but continue with default token estimation
		tokens = 1000 // Conservative default
	}

	// Check rate limits
	delay := h.CheckRateLimit(req, tokens)
	if delay == -1 {
		// Queue full, reject immediately
		w.Header().Set("X-RateLimit-Reason", "queue_full")
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Rate limit exceeded - queue full", http.StatusTooManyRequests)
		return
	}

	if delay > 0 {
		// Add delay header for transparency
		w.Header().Set("X-RateLimit-Delay", delay.String())
		w.Header().Set("X-RateLimit-Reason", "rate_limited")

		// Wait for the delay
		time.Sleep(delay)
	}

	// Add rate limit headers
	w.Header().Set("X-RateLimit-Limit-RPM", strconv.Itoa(h.Config.RPMLimit))
	w.Header().Set("X-RateLimit-Limit-TPM", strconv.Itoa(h.Config.TPMLimit))
	w.Header().Set("X-RateLimit-Queue-Length", strconv.Itoa(h.Limiter.QueueLength()))

	// Add circuit breaker state headers
	stats := h.circuitBreaker.Stats()
	w.Header().Set("X-CircuitBreaker-State", stats.State.String())
	w.Header().Set("X-CircuitBreaker-Failures", strconv.Itoa(stats.Failures))

	// Use circuit breaker to protect the proxy call
	var cbErr error
	cbErr = h.circuitBreaker.Call(func() error {
		// Capture response to detect errors
		responseWriter := &circuitBreakerResponseWriter{
			ResponseWriter: w,
			statusCode:     200,
		}

		// Forward request to Cerebras API
		h.proxy.ServeHTTP(responseWriter, req)

		// Consider HTTP 5xx errors as failures
		if responseWriter.statusCode >= 500 {
			return fmt.Errorf("HTTP %d error from Cerebras API", responseWriter.statusCode)
		}

		return nil
	})

	if cbErr != nil {
		if circuitbreaker.IsCircuitOpenError(cbErr) {
			w.Header().Set("X-CircuitBreaker-Reason", "circuit_open")
			w.Header().Set("Retry-After", "60") // Suggest retry after circuit reset timeout
			http.Error(w, "Cerebras API temporarily unavailable - circuit breaker open", http.StatusServiceUnavailable)
		} else {
			// Other errors (HTTP 5xx from upstream)
			w.Header().Set("X-CircuitBreaker-Reason", "upstream_error")
			http.Error(w, "Cerebras API error", http.StatusBadGateway)
		}
	}
}

func (h *CerebrasProxyHandler) SetTarget(targetURL *url.URL) {
	// Update the proxy director to use the specific target
	h.proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host
	}
}

// CircuitBreakerStats returns current circuit breaker statistics
func (h *CerebrasProxyHandler) CircuitBreakerStats() circuitbreaker.Stats {
	return h.circuitBreaker.Stats()
}

// circuitBreakerResponseWriter wraps an http.ResponseWriter to capture status codes
type circuitBreakerResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *circuitBreakerResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
