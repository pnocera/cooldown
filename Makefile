# Cooldown Proxy Makefile

.PHONY: help build test clean install fmt lint vet coverage benchmark docker-build docker-run

# Default target
help: ## Show this help message
	@echo "Cooldown Proxy - Makefile Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build the binary for current platform
	@echo "Building cooldown-proxy..."
	go build -o cooldown-proxy ./cmd/proxy

build-all: ## Build binaries for all platforms
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build -o cooldown-proxy-linux-amd64 ./cmd/proxy
	GOOS=linux GOARCH=arm64 go build -o cooldown-proxy-linux-arm64 ./cmd/proxy
	GOOS=darwin GOARCH=amd64 go build -o cooldown-proxy-darwin-amd64 ./cmd/proxy
	GOOS=darwin GOARCH=arm64 go build -o cooldown-proxy-darwin-arm64 ./cmd/proxy
	GOOS=windows GOARCH=amd64 go build -o cooldown-proxy-windows-amd64.exe ./cmd/proxy
	@echo "All binaries built successfully!"

# Testing targets
test: ## Run all tests
	@echo "Running tests..."
	go test ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests with verbose output..."
	go test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -cover ./...

test-coverage-html: ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Code quality targets
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

lint: ## Run golint (requires golint to be installed)
	@echo "Running linter..."
	@if command -v golint >/dev/null 2>&1; then \
		golint ./...; \
	else \
		echo "golint not installed. Install with: go install golang.org/x/lint/golint@latest"; \
	fi

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

check: fmt vet test-coverage ## Run all code quality checks

# Installation targets
install: ## Install the binary locally
	@echo "Installing cooldown-proxy..."
	go install ./cmd/proxy

install-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	go install golang.org/x/lint/golint@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Cleanup targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f cooldown-proxy*
	rm -f coverage.out coverage.html
	rm -rf dist/

clean-deps: ## Clean dependency cache
	@echo "Cleaning dependency cache..."
	go clean -modcache

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t cooldown-proxy:latest .

docker-run: docker-build ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 -v $(PWD)/config.yaml.example:/app/config.yaml cooldown-proxy:latest

# Development targets
dev: ## Start development server with config file
	@echo "Starting development server..."
	@if [ ! -f config.yaml ]; then \
		cp config.yaml.example config.yaml; \
		echo "Created config.yaml from example"; \
	fi
	go run ./cmd/proxy -config config.yaml

dev-config: ## Create development config file
	@echo "Creating development config..."
	@cat > config.yaml << 'EOF'
server:
  host: "localhost"
  port: 8080

rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10
  - domain: "*.example.com"
    requests_per_second: 20

default_rate_limit:
  requests_per_second: 5
EOF
	@echo "Development config created: config.yaml"

# Release targets
release: clean test-coverage build-all ## Prepare release
	@echo "Preparing release..."
	@mkdir -p dist
	@mv cooldown-proxy-* dist/
	@echo "Release files prepared in dist/"

# Documentation targets
docs-serve: ## Serve documentation locally (requires mkdocs)
	@echo "Serving documentation..."
	@if command -v mkdocs >/dev/null 2>&1; then \
		mkdocs serve; \
	else \
		echo "mkdocs not installed. Install with: pip install mkdocs"; \
	fi

docs-build: ## Build documentation (requires mkdocs)
	@echo "Building documentation..."
	@if command -v mkdocs >/dev/null 2>&1; then \
		mkdocs build; \
	else \
		echo "mkdocs not installed. Install with: pip install mkdocs"; \
	fi

# CI/CD targets
ci: check ## Run CI checks
	@echo "CI checks completed successfully!"

# Utility targets
version: ## Show version information
	@echo "Cooldown Proxy"
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'not a git repository')"
	@echo "Build date: $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"

deps: ## List dependencies
	@echo "Listing dependencies..."
	go list -m all

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	go mod verify

# Performance targets
prof-cpu: ## Generate CPU profile
	@echo "Generating CPU profile..."
	go run ./cmd/proxy -cpuprofile=cpu.prof

prof-mem: ## Generate memory profile
	@echo "Generating memory profile..."
	go run ./cmd/proxy -memprofile=mem.prof

# Security targets
security-scan: ## Run security scan (requires gosec)
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Quick start targets
quick-start: dev-config dev ## Quick start for development
	@echo "Quick start completed!"
	@echo "Proxy is running at http://localhost:8080"
	@echo "Config file: config.yaml"

# Default values
BINARY_NAME=cooldown-proxy
BUILD_DIR=dist
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')
BUILD_DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

# Advanced build target with build info
build-info: ## Build with version and build info
	@echo "Building with version info..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/proxy
	@echo "Built $(BINARY_NAME) with version $(VERSION), commit $(COMMIT)"