.PHONY: help build build-all build-windows build-linux build-darwin build-loadtest test fmt vet check dev quick-start version clean

# Detect OS and set appropriate binary extension
ifeq ($(OS),Windows_NT)
	BINARY_EXT=.exe
else
	BINARY_EXT=
endif

BINARY_NAME=cooldown-proxy$(BINARY_EXT)
LOADTEST_BINARY=loadtest$(BINARY_EXT)

help:
	@echo "Cooldown Proxy - Available Commands:"
	@echo "  build          - Build the binary for current platform"
	@echo "  build-all      - Build for all platforms"
	@echo "  build-windows  - Build for Windows"
	@echo "  build-linux    - Build for Linux"
	@echo "  build-darwin   - Build for macOS"
	@echo "  build-loadtest - Build load testing tool"
	@echo "  test           - Run tests"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  check          - Run all quality checks"
	@echo "  dev            - Start development server"
	@echo "  quick-start    - Quick start for development"
	@echo "  loadtest       - Run load tests (requires running server)"
	@echo "  version        - Show version info"
	@echo "  clean          - Clean build artifacts"

build:
	@echo "Building cooldown-proxy for $(shell go env GOOS)..."
	go build -o $(BINARY_NAME) ./cmd/proxy
	@echo "Built $(BINARY_NAME)"

build-loadtest:
	@echo "Building loadtest tool for $(shell go env GOOS)..."
	go build -o $(LOADTEST_BINARY) ./cmd/loadtest
	@echo "Built $(LOADTEST_BINARY)"

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build -o cooldown-proxy-windows-amd64.exe ./cmd/proxy
	GOOS=windows GOARCH=arm64 go build -o cooldown-proxy-windows-arm64.exe ./cmd/proxy
	@echo "Built Windows executables"

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o cooldown-proxy-linux-amd64 ./cmd/proxy
	GOOS=linux GOARCH=arm64 go build -o cooldown-proxy-linux-arm64 ./cmd/proxy
	@echo "Built Linux executables"

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o cooldown-proxy-darwin-amd64 ./cmd/proxy
	GOOS=darwin GOARCH=arm64 go build -o cooldown-proxy-darwin-arm64 ./cmd/proxy
	@echo "Built macOS executables"

build-all: build-windows build-linux build-darwin
	@echo "All platform builds completed!"

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

loadtest:
	@echo "Running light load test against localhost:8080..."
	@if [ ! -f $(LOADTEST_BINARY) ]; then $(MAKE) build-loadtest; fi
	./$(LOADTEST_BINARY) -scenario light_load -output loadtest-results.json

clean:
	@echo "Cleaning build artifacts..."
	rm -f cooldown-proxy*
	rm -f cooldown-proxy*.exe
	rm -f loadtest*
	rm -f loadtest*.exe
	rm -f *.json
	@echo "Cleaned build artifacts"

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
	@echo "Go OS: $(shell go env GOOS)"
	@echo "Go ARCH: $(shell go env GOARCH)"
	@echo "Binary extension: $(BINARY_EXT)"
	@echo "Binary name: $(BINARY_NAME)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'not a git repository')"