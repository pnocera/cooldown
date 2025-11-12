# Cooldown Proxy

A local-first reverse proxy with intelligent rate limiting for outgoing REST API requests using the leaky bucket algorithm.

## Features

- **Intelligent Rate Limiting**: Per-domain rate limiting using the leaky bucket algorithm
- **Wildcard Domain Support**: Support for patterns like `*.example.com`
- **Configuration-Driven**: YAML-based configuration with sensible defaults
- **Graceful Shutdown**: Clean shutdown with signal handling
- **Comprehensive Testing**: Full test suite with TDD approach
- **Reverse Proxy**: Forward proxy capabilities with request routing

## Quick Start

### Prerequisites

- Go 1.21+ installed

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd cooldownp

# Build the binary
go build -o cooldown-proxy ./cmd/proxy

# Or install directly
go install ./cmd/proxy
```

### Basic Usage

```bash
# Copy the example configuration
cp config.yaml.example config.yaml

# Start the proxy server
./cooldown-proxy

# Or with custom config file
./cooldown-proxy -config /path/to/config.yaml
```

## Configuration

Create a `config.yaml` file based on `config.yaml.example`:

```yaml
server:
  host: "localhost"    # Server bind address
  port: 8080          # Server port

rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10
  - domain: "api.twitter.com" 
    requests_per_second: 5
  - domain: "*.example.com"    # Wildcard support
    requests_per_second: 20

# Optional: Default rate limit for unspecified domains
default_rate_limit:
  requests_per_second: 1
```

### Configuration Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `server.host` | string | "localhost" | Server bind address |
| `server.port` | int | 8080 | Server port |
| `rate_limits` | array | [] | Array of rate limit rules |
| `rate_limits[].domain` | string | - | Domain pattern (supports wildcards) |
| `rate_limits[].requests_per_second` | int | - | Max requests per second |
| `default_rate_limit.requests_per_second` | int | 1 | Default rate for unknown domains |

## Rate Limiting Algorithm

Cooldown Proxy uses the **leaky bucket algorithm** for rate limiting:

- **Smooth rate limiting**: Requests are evenly distributed over time
- **Burst capacity**: Allows short bursts while maintaining overall rate
- **Per-domain isolation**: Each domain has its own rate limiter
- **Wildcard matching**: Support for patterns like `*.example.com`

### Rate Limiting Behavior

```yaml
rate_limits:
  - domain: "api.example.com"
    requests_per_second: 10  # Allows ~1 request every 100ms
```

- Requests exceeding the rate limit will be delayed
- The delay is automatically calculated to maintain the target rate
- Multiple domains are rate-limited independently

## Usage Examples

### Example 1: API Gateway

```yaml
server:
  host: "0.0.0.0"
  port: 8080

rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10
  - domain: "api.stripe.com"
    requests_per_second: 100
```

### Example 2: Development Environment

```yaml
server:
  host: "localhost"
  port: 3000

rate_limits:
  - domain: "*.dev.example.com"
    requests_per_second: 50
  - domain: "localhost"
    requests_per_second: 1000

default_rate_limit:
  requests_per_second: 5
```

## API Usage

Once running, make requests through the proxy:

```bash
# Direct requests to the proxy
curl -H "Host: api.github.com" http://localhost:8080/users/octocat

# The proxy will apply rate limiting and forward to the target
```

## Development

### Project Structure

```
.
├── cmd/proxy/              # Main application
├── internal/
│   ├── config/            # Configuration management
│   ├── ratelimit/         # Rate limiting implementation
│   ├── proxy/             # HTTP proxy handler
│   └── router/            # Request routing
├── docs/                  # Documentation
├── config.yaml.example    # Example configuration
└── README.md
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/ratelimit/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Building

```bash
# Build for current platform (includes .exe on Windows)
make build

# Build for all platforms
make build-all

# Build for specific platforms
make build-windows    # Creates cooldown-proxy-windows-amd64.exe
make build-linux      # Creates cooldown-proxy-linux-amd64
make build-darwin     # Creates cooldown-proxy-darwin-amd64

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

## Architecture

### Components

1. **Configuration**: YAML-based config loading with validation
2. **Rate Limiter**: Leaky bucket algorithm with domain matching
3. **Proxy Handler**: HTTP reverse proxy with request forwarding
4. **Router**: Domain-based request routing
5. **Server**: HTTP server with graceful shutdown

### Request Flow

1. Request received by HTTP server
2. Router determines target based on Host header
3. Rate limiter applies per-domain rate limiting
4. Proxy handler forwards request to target
5. Response returned to client

## Performance Considerations

- **Memory usage**: O(n) where n is the number of rate limit rules
- **CPU overhead**: Minimal per-request processing
- **Concurrency**: Safe for concurrent use
- **Scalability**: Suitable for moderate traffic loads

## Troubleshooting

### Common Issues

1. **Connection refused**: Check if the target server is accessible
2. **Rate limiting not working**: Verify configuration syntax
3. **High latency**: Consider increasing rate limits for performance testing

### Logging

The proxy provides basic logging for:
- Server startup/shutdown
- Configuration loading
- Error conditions

### Debug Mode

For debugging, you can increase log verbosity by setting environment variables:

```bash
LOG_LEVEL=debug ./cooldown-proxy
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

### Development Guidelines

- Follow Go best practices and idioms
- Write tests for all new functionality
- Use the existing code style and patterns
- Update documentation as needed

## License

[Add your license here]

## Support

For issues and questions:
- Open an issue on GitHub
- Check the troubleshooting section
- Review the configuration examples