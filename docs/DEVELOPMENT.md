# Development Guide

Architecture, testing, and contributing guidelines for Cooldown Proxy.

## Architecture

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

### Project Structure

```
.
├── cmd/
│   ├── proxy/             # Main proxy application
│   │   └── main.go        # Entry point and server setup
│   └── loadtest/          # Load testing CLI tool
│       └── main.go        # Load test scenarios and execution
├── internal/
│   ├── config/            # Configuration management
│   │   ├── types.go       # Configuration types
│   │   └── config.go      # Loading and validation
│   ├── proxy/             # HTTP proxy handler
│   │   ├── proxy.go       # Main proxy implementation
│   │   └── handler.go     # HTTP handler logic
│   ├── ratelimit/         # Rate limiting implementation
│   │   ├── limiter.go     # Leaky bucket implementation
│   │   └── matcher.go     # Domain matching logic
│   └── router/            # Request routing
│       ├── router.go      # Domain-based routing
│       └── target.go      # Target URL resolution
├── docs/                  # Documentation
├── config.yaml.example    # Example configuration
├── Makefile              # Build and development commands
└── README.md
```

## Testing

### Running Tests

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

### Testing Strategy

The project uses TDD approach with comprehensive test coverage:

- **Unit tests** for each internal package
- **Configuration validation tests**
- **Rate limiting algorithm tests**
- **Proxy handler tests**
- **Router functionality tests**
- **Integration tests** for end-to-end scenarios

### Test Structure

```bash
internal/
├── config/
│   └── config_test.go     # Configuration loading and validation
├── proxy/
│   └── proxy_test.go      # Proxy handler functionality
├── ratelimit/
│   ├── limiter_test.go    # Rate limiting algorithm
│   └── matcher_test.go    # Domain matching logic
└── router/
    └── router_test.go     # Request routing
```

## Building

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

### Manual Build Commands

```bash
# Build for current platform
go build -o cooldown-proxy ./cmd/proxy

# Build for specific platforms
GOOS=windows GOARCH=amd64 go build -o cooldown-proxy-windows-amd64.exe ./cmd/proxy
GOOS=linux GOARCH=amd64 go build -o cooldown-proxy-linux-amd64 ./cmd/proxy
GOOS=darwin GOARCH=amd64 go build -o cooldown-proxy-darwin-amd64 ./cmd/proxy
```

## Development Workflow

### Development Commands

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

### Available Commands

| Command | Description |
|---------|-------------|
| `make build` | Build proxy for current platform |
| `make build-all` | Build for all platforms |
| `make test` | Run all tests |
| `make check` | Run format, vet, and tests |
| `make dev` | Start development server |
| `make clean` | Clean build artifacts |

## Code Quality

### Go Best Practices

- Follow Go best practices and idioms
- Use `gofmt` for consistent code formatting
- Run `go vet` to catch potential issues
- Write comprehensive tests for all functionality
- Use interfaces for dependency injection
- Handle errors appropriately

### Code Style

- Use meaningful variable and function names
- Keep functions small and focused
- Add comments for exported functions and complex logic
- Use `// TODO:` comments for temporary solutions
- Follow the existing code patterns and structure

## Dependencies

### Key Dependencies

- `go.uber.org/ratelimit`: Rate limiting implementation
- `gopkg.in/yaml.v3`: YAML configuration parsing
- `github.com/benbjohnson/clock`: Clock utilities for testing

### Adding Dependencies

```bash
# Add new dependency
go get module/path

# Update dependencies
go mod tidy
go mod vendor
```

## Performance Considerations

- **Memory usage**: O(n) where n is the number of rate limit rules
- **CPU overhead**: Minimal per-request processing
- **Concurrency**: Safe for concurrent use
- **Scalability**: Suitable for moderate to high traffic loads

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes
4. Add tests for new functionality
5. Run the test suite: `make check`
6. Commit your changes
7. Push to the feature branch
8. Submit a pull request

### Development Guidelines

- Write tests for all new functionality
- Update documentation as needed
- Use the existing code style and patterns
- Ensure all tests pass before submitting
- Follow conventional commit messages

### Pull Request Process

1. **Description**: Clearly describe what the PR does and why
2. **Testing**: Include tests for new functionality
3. **Documentation**: Update relevant documentation
4. **Review**: Address review comments promptly
5. **Integration**: Ensure CI/CD pipeline passes

## Debugging

### Logging

The proxy provides basic logging for:
- Server startup/shutdown
- Configuration loading
- Error conditions

### Debug Mode

For debugging, you can increase log verbosity:

```bash
LOG_LEVEL=debug ./cooldown-proxy
```

### Common Issues

1. **Connection refused**: Check if the target server is accessible
2. **Rate limiting not working**: Verify configuration syntax
3. **High latency**: Consider increasing rate limits for performance testing