package modelrouting

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

// Metrics tracks model routing statistics
type Metrics struct {
	RoutingAttempts      int64
	RoutingSuccess       int64
	RoutingFallback      int64
	ParsingErrors        int64
	TotalProcessingTimeNs int64 // Stored as nanoseconds for atomic operations
}

type ModelRoutingMiddleware struct {
	config      *config.ModelRoutingConfig
	nextHandler http.Handler
	logger      *log.Logger
	metrics     *Metrics
}

func NewModelRoutingMiddleware(cfg *config.ModelRoutingConfig, next http.Handler) *ModelRoutingMiddleware {
	return &ModelRoutingMiddleware{
		config:      cfg,
		nextHandler: next,
		logger:      log.New(log.Writer(), "[model-routing] ", log.LstdFlags),
		metrics:     &Metrics{},
	}
}

func (m *ModelRoutingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		processingTimeNs := time.Since(start).Nanoseconds()
		atomic.AddInt64(&m.metrics.TotalProcessingTimeNs, processingTimeNs)
	}()

	if !m.shouldApplyRouting(r) {
		m.nextHandler.ServeHTTP(w, r)
		return
	}

	atomic.AddInt64(&m.metrics.RoutingAttempts, 1)

	target, err := m.extractTargetFromModel(r)
	if err != nil || target == "" {
		// Fallback to default target on any error
		target = m.config.DefaultTarget
		atomic.AddInt64(&m.metrics.RoutingFallback, 1)
		if err != nil {
			m.logger.Printf("Model routing failed, using default: %v", err)
			atomic.AddInt64(&m.metrics.ParsingErrors, 1)
		}
	} else {
		atomic.AddInt64(&m.metrics.RoutingSuccess, 1)
	}

	if target != "" {
		m.rewriteRequest(r, target)
		m.logger.Printf("Routed request to: %s", target)
	}

	m.nextHandler.ServeHTTP(w, r)
}

func (m *ModelRoutingMiddleware) shouldApplyRouting(r *http.Request) bool {
	if m.config == nil || !m.config.Enabled {
		return false
	}

	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

func (m *ModelRoutingMiddleware) extractTargetFromModel(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}

	// Create TeeReader to stream while parsing
	var buf bytes.Buffer
	tee := io.TeeReader(r.Body, &buf)

	// Replace body with buffered content for downstream handlers
	r.Body = io.NopCloser(&buf)

	// Parse JSON to extract model field
	return m.parseModelField(tee)
}

func (m *ModelRoutingMiddleware) parseModelField(reader io.Reader) (string, error) {
	// Read the entire request body into a map to find the model field
	// This is simpler and more reliable than streaming parsing for this use case
	var data map[string]interface{}
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return "", err
	}

	// Extract the model field
	if modelValue, ok := data["model"].(string); ok {
		return m.config.Models[modelValue], nil
	}

	return "", nil
}

// ParseModelField is a public method for testing parseModelField
func (m *ModelRoutingMiddleware) ParseModelField(reader io.Reader) (string, error) {
	return m.parseModelField(reader)
}

// GetMetrics returns a copy of the current metrics
func (m *ModelRoutingMiddleware) GetMetrics() Metrics {
	return Metrics{
		RoutingAttempts:      atomic.LoadInt64(&m.metrics.RoutingAttempts),
		RoutingSuccess:       atomic.LoadInt64(&m.metrics.RoutingSuccess),
		RoutingFallback:      atomic.LoadInt64(&m.metrics.RoutingFallback),
		ParsingErrors:        atomic.LoadInt64(&m.metrics.ParsingErrors),
		TotalProcessingTimeNs: atomic.LoadInt64(&m.metrics.TotalProcessingTimeNs),
	}
}

// HealthCheck returns health status information for the model routing middleware
func (m *ModelRoutingMiddleware) HealthCheck() map[string]interface{} {
	metrics := m.GetMetrics()

	attempts := metrics.RoutingAttempts
	success := metrics.RoutingSuccess
	fallback := metrics.RoutingFallback
	errors := metrics.ParsingErrors

	var successRate float64
	if attempts > 0 {
		successRate = float64(success) / float64(attempts) * 100
	}

	var avgProcessingTime float64
	if attempts > 0 {
		avgProcessingTime = float64(metrics.TotalProcessingTimeNs) / float64(attempts) / 1e6 // Convert to milliseconds
	}

	var status string
	if m.config == nil || !m.config.Enabled {
		status = "disabled"
	} else if successRate >= 95 {
		status = "healthy"
	} else if successRate >= 80 {
		status = "degraded"
	} else {
		status = "unhealthy"
	}

	result := map[string]interface{}{
		"status":              status,
		"enabled":             m.config != nil && m.config.Enabled,
		"routing_attempts":   attempts,
		"routing_success":    success,
		"routing_fallback":   fallback,
		"parsing_errors":     errors,
		"success_rate":       successRate,
		"avg_processing_ms": avgProcessingTime,
	}

	// Only add config fields if config exists
	if m.config != nil {
		result["models_configured"] = len(m.config.Models)
		result["default_target"] = m.config.DefaultTarget
	} else {
		result["models_configured"] = 0
		result["default_target"] = ""
	}

	return result
}

func (m *ModelRoutingMiddleware) rewriteRequest(r *http.Request, target string) {
	targetURL, err := url.Parse(target)
	if err != nil {
		m.logger.Printf("Invalid target URL %s: %v", target, err)
		return
	}

	r.URL.Scheme = targetURL.Scheme
	r.URL.Host = targetURL.Host
	r.Host = targetURL.Host
}