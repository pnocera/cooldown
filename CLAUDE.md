# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cooldown Proxy is a local-first reverse proxy with intelligent rate limiting for outgoing REST API requests using the leaky bucket algorithm. It's written in Go and provides per-domain rate limiting with wildcard domain support.

## Development Commands

### Build Commands
```bash
# Build for current platform
make build
# or
go build -o cooldown-proxy ./cmd/proxy

# Build for all platforms
make build-all

# Build for specific platforms
make build-windows    # Windows executables
make build-linux      # Linux executables  
make build-darwin     # macOS executables

# Clean build artifacts
make clean
```

### Testing Commands
```bash
# Run all tests
make test
# or
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/ratelimit/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Development Workflow
```bash
# Start development server (creates config.yaml if missing)
make dev
# or
make quick-start

# Run all quality checks (format, vet, test)
make check

# Format code
make fmt

# Run go vet
make vet
```

### Running the Application
```bash
# Run with default config
./cooldown-proxy

# Run with custom config
./cooldown-proxy -config /path/to/config.yaml

# Development server with auto-config creation
make dev
```

## Architecture

The project follows a clean architecture pattern with clear separation of concerns:

### Core Components

1. **Configuration** (`internal/config/`):
   - YAML-based configuration loading and validation
   - Types defined in `types.go`: `Config`, `ServerConfig`, `RateLimitRule`
   - Supports per-domain rate limits with wildcard patterns

2. **Rate Limiter** (`internal/ratelimit/`):
   - Implements leaky bucket algorithm using `go.uber.org/ratelimit`
   - Per-domain rate limiting with wildcard matching (e.g., `*.example.com`)
   - Default rate limiter for unspecified domains
   - Main interface: `GetDelay(domain string) time.Duration`

3. **Proxy Handler** (`internal/proxy/`):
   - HTTP reverse proxy using `net/http/httputil.ReverseProxy`
   - Integrates rate limiting before forwarding requests
   - Dynamic target URL setting via `SetTarget()`
   - Main entry point: `ServeHTTP(w http.ResponseWriter, r *http.Request)`

4. **Router** (`internal/router/`):
   - Domain-based request routing
   - Maps host headers to target URLs
   - Integrates with proxy handler for request forwarding
   - Returns 404 for unknown hosts (security measure)

5. **Main Application** (`cmd/proxy/main.go`):
   - Configuration loading from command-line flags
   - Graceful shutdown with signal handling
   - HTTP server setup with middleware integration

### Request Flow
1. HTTP request received by server
2. Router determines target based on `Host` header
3. Rate limiter applies per-domain rate limiting
4. Proxy handler forwards request to target
5. Response returned to client

### Configuration Structure
- `server.host`/`server.port`: Server bind address and port
- `rate_limits[]`: Array of domain-specific rate limit rules
- `rate_limits[].domain`: Domain pattern (supports wildcards like `*.example.com`)
- `rate_limits[].requests_per_second`: Max requests per second per domain
- `default_rate_limit`: Fallback rate limit for unspecified domains

## Key Dependencies

- `go.uber.org/ratelimit`: Rate limiting implementation
- `gopkg.in/yaml.v3`: YAML configuration parsing
- `github.com/benbjohnson/clock`: Clock utilities for testing

## Testing Strategy

The project uses TDD approach with comprehensive test coverage:
- Unit tests for each internal package
- Configuration validation tests
- Rate limiting algorithm tests
- Proxy handler tests
- Router functionality tests

## Configuration Files

- `config.yaml.example`: Example configuration template
- `config.yaml`: Runtime configuration (created from example during development)
- Configuration is hot-reloadable via server restart