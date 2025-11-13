package modelrouting

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

type ModelRoutingMiddleware struct {
	config      *config.ModelRoutingConfig
	nextHandler http.Handler
	logger      *log.Logger
}

func NewModelRoutingMiddleware(cfg *config.ModelRoutingConfig, next http.Handler) *ModelRoutingMiddleware {
	return &ModelRoutingMiddleware{
		config:      cfg,
		nextHandler: next,
		logger:      log.New(log.Writer(), "[model-routing] ", log.LstdFlags),
	}
}

func (m *ModelRoutingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.shouldApplyRouting(r) {
		m.nextHandler.ServeHTTP(w, r)
		return
	}

	target, err := m.extractTargetFromModel(r)
	if err != nil || target == "" {
		// Fallback to default target on any error
		target = m.config.DefaultTarget
		if err != nil {
			m.logger.Printf("Model routing failed, using default: %v", err)
		}
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