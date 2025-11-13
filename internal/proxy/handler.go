package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/proxyerrors"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
)

type Handler struct {
	reverseProxy *httputil.ReverseProxy
	rateLimiter  *ratelimit.Limiter
	logger       *log.Logger
	targetURL    *url.URL
}

func NewHandler(rateLimiter *ratelimit.Limiter) *Handler {
	return &Handler{
		reverseProxy: &httputil.ReverseProxy{},
		rateLimiter:  rateLimiter,
		logger:       log.New(log.Writer(), "[proxy] ", log.LstdFlags),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a response writer wrapper to capture errors
	rw := &responseWriter{ResponseWriter: w}

	// Apply rate limiting based on host
	if err := h.applyRateLimiting(r); err != nil {
		h.handleError(rw, r, err)
		return
	}

	// Validate request
	if err := h.validateRequest(r); err != nil {
		h.handleError(rw, r, err)
		return
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)

	// Set target if configured
	if h.targetURL != nil {
		h.setTargetRequest(r, h.targetURL)
	}

	// Proxy the request with error handling
	h.serveProxyWithRetry(rw, r)
}

func (h *Handler) applyRateLimiting(r *http.Request) error {
	if h.rateLimiter == nil {
		return nil
	}

	delay := h.rateLimiter.GetDelay(r.Host)
	if delay > 0 {
		// Check if the delay would exceed our timeout context
		ctx := r.Context()
		select {
		case <-ctx.Done():
			return proxyerrors.NewUpstreamTimeoutError(r.Host, ctx.Err())
		case <-time.After(delay):
			// Rate limit delay applied, continue
			h.logger.Printf("Rate limited request to %s, delayed by %v", r.Host, delay)
		}
	}

	return nil
}

func (h *Handler) validateRequest(r *http.Request) error {
	// Validate that the request has a valid Host header
	if r.Host == "" {
		return proxyerrors.NewInvalidRequestError("missing Host header")
	}

	// Validate that the URL is properly formed
	if r.URL == nil {
		return proxyerrors.NewInvalidRequestError("invalid request URL")
	}

	// Prevent request smuggling attempts
	if strings.Contains(r.URL.Path, "..") {
		return proxyerrors.NewInvalidRequestError("invalid path: directory traversal not allowed")
	}

	return nil
}

func (h *Handler) serveProxyWithRetry(w *responseWriter, r *http.Request) {
	// Create a custom error handler for the reverse proxy
	h.reverseProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		h.logger.Printf("Proxy error for %s: %v", req.URL.Path, err)

		// Determine error type and create appropriate error
		var proxyErr *proxyerrors.ProxyError
		if netErr, ok := err.(net.Error); ok {
			if netErr.Timeout() {
				proxyErr = proxyerrors.NewUpstreamTimeoutError(req.Host, err)
			} else if netErr.Temporary() {
				proxyErr = proxyerrors.NewUpstreamConnectionError(req.Host, err)
			} else {
				proxyErr = proxyerrors.NewUpstreamConnectionError(req.Host, err)
			}
		} else {
			proxyErr = proxyerrors.NewUpstreamConnectionError(req.Host, err)
		}

		h.handleError(w, req, proxyErr)
	}

	// Create a custom modify response to handle upstream responses
	h.reverseProxy.ModifyResponse = func(resp *http.Response) error {
		// Log response status
		h.logger.Printf("Upstream response: %d %s for %s", resp.StatusCode, resp.Status, resp.Request.URL.Path)

		// Handle specific error status codes
		if resp.StatusCode >= 500 {
			// Server error from upstream
			return fmt.Errorf("upstream server error: %d", resp.StatusCode)
		}

		return nil
	}

	// Serve the request
	h.reverseProxy.ServeHTTP(w, r)
}

func (h *Handler) handleError(w *responseWriter, r *http.Request, err error) {
	var proxyErr *proxyerrors.ProxyError
	if errors.As(err, &proxyErr) {
		// It's already a ProxyError
	} else {
		// Wrap as internal error
		proxyErr = proxyerrors.NewInternalError("unexpected error", err)
	}

	// Log the error
	h.logger.Printf("Proxy error for %s %s: %v", r.Method, r.URL.Path, proxyErr)

	// Set appropriate status code
	if !w.wroteHeader {
		w.WriteHeader(proxyErr.HTTPStatus())
	}

	// Write error response
	w.Header().Set("Content-Type", "application/json")

	// Write JSON response
	jsonBytes := []byte(fmt.Sprintf(`{"error":%d,"message":"%s","status":%d}`,
		proxyErr.Type, proxyErr.Message, proxyErr.HTTPStatus()))
	w.Write(jsonBytes)
}

func (h *Handler) SetTarget(targetURL *url.URL) {
	h.targetURL = targetURL
}

func (h *Handler) setTargetRequest(r *http.Request, targetURL *url.URL) {
	// Update the director to use the specific target
	h.reverseProxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host

		// Set X-Forwarded headers
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "https")
		if req.URL.Scheme != "" {
			req.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
		}
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}
}

// responseWriter wraps http.ResponseWriter to track if headers have been written
type responseWriter struct {
	http.ResponseWriter
	wroteHeader bool
	statusCode   int
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.wroteHeader = true
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
		rw.wroteHeader = true
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Status() int {
	if !rw.wroteHeader {
		return http.StatusOK
	}
	return rw.statusCode
}
