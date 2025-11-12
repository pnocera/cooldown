package proxy

import (
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type Handler struct {
	reverseProxy *httputil.ReverseProxy
	rateLimiter  *ratelimit.Limiter
}

func NewHandler(rateLimiter *ratelimit.Limiter) *Handler {
	director := func(req *http.Request) {
		// For now, use a simple passthrough
		// Target URL will be set by router
	}

	return &Handler{
		reverseProxy: &httputil.ReverseProxy{Director: director},
		rateLimiter:  rateLimiter,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply rate limiting based on host
	if h.rateLimiter != nil {
		delay := h.rateLimiter.GetDelay(r.Host)
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	h.reverseProxy.ServeHTTP(w, r)
}

func (h *Handler) SetTarget(targetURL *url.URL) {
	// Update the director to use the specific target
	h.reverseProxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host
	}
}
