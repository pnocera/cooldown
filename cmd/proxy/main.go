package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/modelrouting"
	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/router"
	"github.com/cooldownp/cooldown-proxy/internal/token"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting cooldown proxy on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Initialize rate limiter
	rateLimiter := ratelimit.New(cfg.RateLimits)
	if cfg.DefaultRateLimit != nil {
		// Set default rate limit
		// This will need to be implemented in the rate limiter
	}

	// Initialize Cerebras components if configured (check if RPM limit is set)
	var cerebrasHandler *proxy.CerebrasProxyHandler
	if cfg.CerebrasLimits.RPMLimit > 0 {
		log.Printf("Initializing Cerebras rate limiting with RPM: %d, TPM: %d",
			cfg.CerebrasLimits.RPMLimit, cfg.CerebrasLimits.TPMLimit)

		// Initialize Cerebras-specific components
		cerebrasLimiter := ratelimit.NewCerebrasLimiter(cfg.CerebrasLimits.RPMLimit, cfg.CerebrasLimits.TPMLimit)
		tokenEstimator := token.NewTokenEstimator()
		cerebrasHandler = proxy.NewCerebrasProxyHandler(cerebrasLimiter, tokenEstimator, &cfg.CerebrasLimits)

		log.Printf("Cerebras handler initialized with queue depth: %d, timeout: %v",
			cfg.CerebrasLimits.MaxQueueDepth, cfg.CerebrasLimits.RequestTimeout)
	}

	// Initialize model routing if configured
	var modelRoutingMiddleware *modelrouting.ModelRoutingMiddleware
	if cfg.ModelRouting != nil && cfg.ModelRouting.Enabled {
		log.Printf("Initializing model routing with %d models configured", len(cfg.ModelRouting.Models))
		modelRoutingMiddleware = modelrouting.NewModelRoutingMiddleware(cfg.ModelRouting, nil)
	}

	// Initialize router
	routes := make(map[string]*url.URL)
	// TODO: Load routes from configuration or use direct passthrough

	r := router.New(routes, rateLimiter)

	// Create composite handler that routes between all handlers appropriately
	compositeHandler := &CompositeHandler{
		cerebrasHandler: cerebrasHandler,
		standardRouter:  r,
		modelRouting:    modelRoutingMiddleware,
	}

	// If model routing is enabled, wrap it properly
	if modelRoutingMiddleware != nil {
		// Create base handler chain
		baseHandler := &CompositeHandler{
			cerebrasHandler: cerebrasHandler,
			standardRouter:  r,
			modelRouting:    nil, // Don't recurse
		}

		// Wrap base handler with model routing
		modelRoutingMiddleware = modelrouting.NewModelRoutingMiddleware(cfg.ModelRouting, baseHandler)

		compositeHandler = &CompositeHandler{
			cerebrasHandler: nil, // Handled by baseHandler
			standardRouter:  nil, // Handled by baseHandler
			modelRouting:    modelRoutingMiddleware,
		}
	}

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: compositeHandler,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// CompositeHandler routes requests between Cerebras handler, model routing, and standard router
type CompositeHandler struct {
	cerebrasHandler *proxy.CerebrasProxyHandler
	standardRouter  http.Handler
	modelRouting    *modelrouting.ModelRoutingMiddleware
}

func (ch *CompositeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply model routing first if enabled
	if ch.modelRouting != nil {
		ch.modelRouting.ServeHTTP(w, r)
		return
	}

	// Route Cerebras requests to the special handler if configured
	if ch.cerebrasHandler != nil && ch.isCerebrasRequest(r) {
		ch.cerebrasHandler.ServeHTTP(w, r)
		return
	}

	// Route all other requests to the standard router
	ch.standardRouter.ServeHTTP(w, r)
}

func (ch *CompositeHandler) isCerebrasRequest(req *http.Request) bool {
	host := req.Host
	// Remove port if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Check against known Cerebras hosts
	cerebrasHosts := []string{
		"api.cerebras.ai",
		"inference.cerebras.ai",
	}

	for _, cerebrasHost := range cerebrasHosts {
		if host == cerebrasHost {
			return true
		}
	}

	return false
}
