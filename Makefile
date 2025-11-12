.PHONY: help build test fmt vet check dev quick-start version

help:
	@echo "Cooldown Proxy - Available Commands:"
	@echo "  build        - Build the binary"
	@echo "  test         - Run tests"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  check        - Run all quality checks"
	@echo "  dev          - Start development server"
	@echo "  quick-start  - Quick start for development"
	@echo "  version      - Show version info"

build:
	@echo "Building cooldown-proxy..."
	go build -o cooldown-proxy ./cmd/proxy

test:
	@echo "Running tests..."
	go test ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

check: fmt vet test

dev:
	@echo "Starting development server..."
	@if [ ! -f config.yaml ]; then \
		cp config.yaml.example config.yaml; \
		echo "Created config.yaml from example"; \
	fi
	go run ./cmd/proxy -config config.yaml

quick-start: dev
	@echo "Quick start completed!"
	@echo "Proxy is running at http://localhost:8080"

version:
	@echo "Cooldown Proxy"
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'not a git repository')"