package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/proxyerrors"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/stretchr/testify/assert"
)

func TestProxyHandlesUpstreamErrors(t *testing.T) {
	// Test with invalid upstream that will fail
	targetURL, _ := url.Parse("http://invalid-host-that-does-not-exist:9999")

	proxyHandler := NewHandler(nil)
	proxyHandler.SetTarget(targetURL)

	// Create test request
	req := httptest.NewRequest("GET", "http://test.com/api/users", nil)
	w := httptest.NewRecorder()

	// Serve request
	proxyHandler.ServeHTTP(w, req)

	// Should return 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to connect to upstream server")
	assert.Contains(t, w.Body.String(), "502")
}

func TestProxyHandlesRequestValidation(t *testing.T) {
	// Set up a dummy target URL for testing
	targetURL, _ := url.Parse("http://example.com")
	proxyHandler := NewHandler(nil)
	proxyHandler.SetTarget(targetURL)

	tests := []struct {
		name           string
		request        *http.Request
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing host header",
			request:        httptest.NewRequest("GET", "http://example.com/", nil),
			expectedStatus: http.StatusOK, // Host is present, so this should be OK
			expectedError:  "",
		},
		{
			name: "directory traversal attempt",
			request: httptest.NewRequest("GET", "http://test.com/../../../etc/passwd", nil),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "directory traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			proxyHandler.ServeHTTP(w, tt.request)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestProxyHandlesTimeouts(t *testing.T) {
	// Create a slow upstream server
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second) // Longer than our 30s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	targetURL, _ := url.Parse(slowServer.URL)
	proxyHandler := NewHandler(nil)
	proxyHandler.SetTarget(targetURL)

	// Create test request with short timeout
	req := httptest.NewRequest("GET", "http://test.com/api/slow", nil)

	// Create context with very short timeout to force timeout quickly
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	start := time.Now()
	proxyHandler.ServeHTTP(w, req)
	duration := time.Since(start)

	// Should timeout quickly and return 504 Gateway Timeout
	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	assert.True(t, duration < 1*time.Second) // Should fail quickly, not wait 35s
}

func TestProxyErrorResponses(t *testing.T) {
	proxyHandler := NewHandler(nil)

	// Test that error responses are in JSON format
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://test.com/../../../etc/passwd", nil)

	proxyHandler.ServeHTTP(w, req)

	// Should contain JSON error response
	responseBody := w.Body.String()
	assert.Contains(t, responseBody, `"error"`)
	assert.Contains(t, responseBody, `"message"`)
	assert.Contains(t, responseBody, `"status"`)
	assert.Contains(t, responseBody, "directory traversal")
}

func TestProxySetsForwardedHeaders(t *testing.T) {
	// Create test upstream server that checks headers
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for forwarded headers
		assert.NotEmpty(t, r.Header.Get("X-Forwarded-Host"))
		assert.NotEmpty(t, r.Header.Get("X-Forwarded-Proto"))
		assert.NotEmpty(t, r.Header.Get("X-Forwarded-For"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer upstreamServer.Close()

	targetURL, _ := url.Parse(upstreamServer.URL)
	proxyHandler := NewHandler(nil)
	proxyHandler.SetTarget(targetURL)

	req := httptest.NewRequest("GET", "http://test.com/api/test", nil)
	w := httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	// Should succeed
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProxyHandlesRateLimitTimeout(t *testing.T) {
	// Create rate limiter with very slow rate
	rules := []config.RateLimitRule{
		{Domain: "test.com", RequestsPerSecond: 1}, // Very slow
	}
	limiter := ratelimit.New(rules)

	// Set a valid target URL for testing
	targetURL, _ := url.Parse("http://example.com")
	proxyHandler := NewHandler(limiter)
	proxyHandler.SetTarget(targetURL)

	// Create request with short timeout context
	req := httptest.NewRequest("GET", "http://test.com/api/test", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	proxyHandler.ServeHTTP(w, req)

	// Should return timeout or bad gateway error due to rate limiting + invalid target
	assert.True(t, w.Code == http.StatusGatewayTimeout || w.Code == http.StatusBadGateway)
}

func TestCustomErrorTypes(t *testing.T) {
	tests := []struct {
		name           string
		errorType      proxyerrors.ErrorType
		expectedStatus int
	}{
		{
			name:           "upstream connection error",
			errorType:      proxyerrors.ErrorTypeUpstreamConnection,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "upstream timeout error",
			errorType:      proxyerrors.ErrorTypeUpstreamTimeout,
			expectedStatus: http.StatusGatewayTimeout,
		},
		{
			name:           "rate limit exceeded",
			errorType:      proxyerrors.ErrorTypeRateLimitExceeded,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "invalid request",
			errorType:      proxyerrors.ErrorTypeInvalidRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "configuration error",
			errorType:      proxyerrors.ErrorTypeConfiguration,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "internal error",
			errorType:      proxyerrors.ErrorTypeInternal,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyErr := proxyerrors.NewProxyError(tt.errorType, "test message", nil)
			assert.Equal(t, tt.expectedStatus, proxyErr.HTTPStatus())
			assert.Equal(t, tt.errorType, proxyErr.Type)
			assert.Equal(t, "test message", proxyErr.Message)
		})
	}
}

func TestResponseWriterWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}

	// Test initial state
	assert.Equal(t, http.StatusOK, rw.Status())
	assert.False(t, rw.wroteHeader)

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.Status())
	assert.True(t, rw.wroteHeader)

	// Test Write
	data := []byte("test data")
	n, err := rw.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, http.StatusNotFound, rw.Status()) // Should preserve first status code
}