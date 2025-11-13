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
	"syscall"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/handler"
	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/router"
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

	// Create rate limiter for OpenAI handler
	rateLimiter := ratelimit.New(cfg.RateLimits)

	// Create handlers
	anthropicHandler := handler.NewAnthropicHandler(cfg)
	openaiHandler := proxy.NewHandler(rateLimiter) // existing OpenAI-compatible handler

	// Create router with default routes
	routes := make(map[string]*url.URL)
	mainRouter := router.New(routes, rateLimiter)

	// Setup routes
	mux := http.NewServeMux()

	// Anthropic endpoint for Claude Code
	anthropicPath := cfg.Server.AnthropicEndpoint
	if anthropicPath == "" {
		anthropicPath = "/anthropic"
	}
	mux.Handle(anthropicPath+"/", http.StripPrefix(anthropicPath, anthropicHandler))

	// OpenAI-compatible endpoint
	openaiPath := cfg.Server.OpenAIEndpoint
	if openaiPath == "" {
		openaiPath = "/openai"
	}
	mux.Handle(openaiPath+"/", http.StripPrefix(openaiPath, openaiHandler))

	// Default proxy routes (existing behavior)
	mux.Handle("/", mainRouter)

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	log.Printf("Starting server on %s", addr)
	log.Printf("Anthropic endpoint: http://%s%s", addr, anthropicPath)
	log.Printf("OpenAI endpoint: http://%s%s", addr, openaiPath)

	go func() {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

