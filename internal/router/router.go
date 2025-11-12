package router

import (
	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"net/http"
	"net/url"
)

type Router struct {
	routes       map[string]*url.URL
	proxyHandler *proxy.Handler
	rateLimiter  *ratelimit.Limiter
}

func New(routes map[string]*url.URL, rateLimiter *ratelimit.Limiter) *Router {
	return &Router{
		routes:       routes,
		proxyHandler: proxy.NewHandler(rateLimiter),
		rateLimiter:  rateLimiter,
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Extract target from request
	targetURL := r.getTarget(req)
	if targetURL == nil {
		http.Error(w, "No route found for host: "+req.Host, http.StatusNotFound)
		return
	}

	// Update proxy handler with target
	r.proxyHandler.SetTarget(targetURL)

	// Serve through proxy
	r.proxyHandler.ServeHTTP(w, req)
}

func (r *Router) getTarget(req *http.Request) *url.URL {
	// Try exact host match
	if target, exists := r.routes[req.Host]; exists {
		return target
	}

	// Only try direct passthrough if the URL is absolute and valid
	if req.URL != nil && req.URL.Host != "" && req.URL.Scheme != "" {
		// For security, don't allow arbitrary passthrough unless we have explicit routes
		// Return nil so we get 404 for unknown hosts
		return nil
	}

	return nil
}
