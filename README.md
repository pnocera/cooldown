# Cooldown Proxy

A local-first reverse proxy with intelligent rate limiting for outgoing REST API requests using the leaky bucket algorithm.

## Features

### Core Functionality
- **Intelligent Rate Limiting**: Per-domain rate limiting using the leaky bucket algorithm
- **Wildcard Domain Support**: Support for patterns like `*.example.com`
- **Configuration-Driven**: YAML-based configuration with sensible defaults
- **Graceful Shutdown**: Clean shutdown with signal handling
- **Comprehensive Testing**: Full test suite with TDD approach
- **Reverse Proxy**: Forward proxy capabilities with request routing

### 🚀 **NEW: Cerebras AI Rate Limiting**
- **Dual-Metric Enforcement**: RPM (Requests Per Minute) + TPM (Tokens Per Minute)
- **Intelligent Queuing**: Priority-based request queuing with smart sorting
- **Circuit Breaker Protection**: Prevents cascading failures with automatic recovery
- **Token Estimation**: Automatic token counting from request payloads
- **Load Testing Framework**: Built-in performance testing tools
- **Real-time Monitoring**: Comprehensive headers for debugging and monitoring

## Quick Start

### Prerequisites

- Go 1.21+ installed

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd cooldownp

# Build the binary and load testing tool
make build
make build-loadtest

# Or install directly
go install ./cmd/proxy
go install ./cmd/loadtest
```

### Basic Usage

#### Standard Rate Limiting
```bash
# Copy the example configuration
cp config.yaml.example config.yaml

# Start the proxy server
./cooldown-proxy

# Or with custom config file
./cooldown-proxy -config /path/to/config.yaml
```

#### Cerebras AI Rate Limiting
```bash
# Configure Cerebras limits in config.yaml
# (See configuration example below)

# Start with Cerebras support enabled
./cooldown-proxy

# Make requests to Cerebras through the proxy
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Host: api.cerebras.ai" \
  -H "Content-Type: application/json" \
  -d '{"model": "llama3.1-8b", "messages": [{"role": "user", "content": "Hello!"}]}'
```

### Load Testing
```bash
# List available load test scenarios
./loadtest -list

# Run a light load test
./loadtest -scenario light_load

# Run custom test with results
./loadtest -clients 100 -rate 500 -duration 5m -output results.json
```

## Configuration

Create a `config.yaml` file based on `config.yaml.example`:

### Standard Rate Limiting
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

### Cerebras AI Rate Limiting (NEW)
```yaml
server:
  host: "localhost"
  port: 8080

# Standard rate limits for other APIs
rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10

# Cerebras AI specific rate limiting configuration
cerebras_limits:
  rpm_limit: 1000              # Requests per minute limit
  tpm_limit: 1000000           # Tokens per minute limit
  max_queue_depth: 100         # Maximum queued requests
  request_timeout: 10m         # Maximum queue wait time
  priority_threshold: 0.7      # Priority adjustment threshold (70%)
```

### Configuration Options

#### Standard Settings
| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `server.host` | string | "localhost" | Server bind address |
| `server.port` | int | 8080 | Server port |
| `rate_limits` | array | [] | Array of rate limit rules |
| `rate_limits[].domain` | string | - | Domain pattern (supports wildcards) |
| `rate_limits[].requests_per_second` | int | - | Max requests per second |
| `default_rate_limit.requests_per_second` | int | 1 | Default rate for unknown domains |

#### Cerebras AI Settings (NEW)
| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `cerebras_limits.rpm_limit` | int | 1000 | Requests per minute limit |
| `cerebras_limits.tpm_limit` | int | 1000000 | Tokens per minute limit |
| `cerebras_limits.max_queue_depth` | int | 100 | Maximum queued requests |
| `cerebras_limits.request_timeout` | duration | 10m | Maximum queue wait time |
| `cerebras_limits.priority_threshold` | float64 | 0.7 | Priority adjustment threshold |

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
├── cmd/
│   ├── proxy/             # Main proxy application
│   └── loadtest/          # Load testing CLI tool
├── internal/
│   ├── config/            # Configuration management
│   ├── circuitbreaker/    # Circuit breaker implementation
│   ├── loadtest/          # Load testing framework
│   ├── proxy/             # HTTP proxy handler
│   ├── queue/             # Priority queue system
│   ├── ratelimit/         # Rate limiting implementation
│   ├── router/            # Request routing
│   └── token/             # Token estimation
├── docs/                  # Documentation
│   ├── CEREBRAS_RATE_LIMITING.md
│   └── LOAD_TESTING.md
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
# Build proxy and load testing tool for current platform
make build
make build-loadtest

# Build for all platforms
make build-all

# Build for specific platforms
make build-windows    # Creates cooldown-proxy-windows-amd64.exe
make build-linux      # Creates cooldown-proxy-linux-amd64
make build-darwin     # Creates cooldown-proxy-darwin-amd64

# Load testing
make loadtest         # Runs light load test

# Clean build artifacts
make clean
```

### Available Commands

| Command | Description |
|---------|-------------|
| `make build` | Build proxy for current platform |
| `make build-loadtest` | Build load testing tool |
| `make build-all` | Build for all platforms |
| `make test` | Run all tests |
| `make check` | Run format, vet, and tests |
| `make loadtest` | Run load test against localhost:8080 |
| `make dev` | Start development server |
| `make clean` | Clean build artifacts |

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
3. **Cerebras Rate Limiter**: Dual-metric (RPM/TPM) intelligent rate limiting
4. **Priority Queue**: Smart request queuing with priority sorting
5. **Circuit Breaker**: Failure detection and automatic recovery
6. **Token Estimator**: Automatic token counting from request payloads
7. **Proxy Handler**: HTTP reverse proxy with request forwarding
8. **Router**: Domain-based request routing with Cerebras detection
9. **Server**: HTTP server with graceful shutdown

### Request Flow

#### Standard APIs
1. Request received by HTTP server
2. Router determines target based on Host header
3. Rate limiter applies per-domain rate limiting
4. Proxy handler forwards request to target
5. Response returned to client

#### Cerebras AI APIs
1. Request received by HTTP server
2. Router detects Cerebras API request
3. Token estimator counts tokens from request payload
4. Cerebras rate limiter applies RPM/TPM limits
5. Intelligent queuing with priority calculation
6. Circuit breaker protects against failures
7. Proxy handler forwards request to target
8. Response returned with comprehensive monitoring headers

## Performance Considerations

- **Memory usage**: O(n) where n is the number of rate limit rules
- **CPU overhead**: Minimal per-request processing
- **Concurrency**: Safe for concurrent use
- **Scalability**: Suitable for moderate to high traffic loads
- **Throughput**: 1000+ RPS with <100ms latency (depending on configuration)
- **Queue Efficiency**: O(log n) priority queue operations
- **Circuit Breaker**: O(1) state tracking and failure detection

### Performance Tiers

| Configuration | Expected RPS | Latency | Use Case |
|---------------|-------------|---------|---------|
| Conservative | 10-100 | 500ms-2s | Development |
| Standard | 100-500 | 100-500ms | Staging |
| Performance | 500-1000 | 50-200ms | Production |
| High-Performance | 1000+ | <100ms | High-traffic |

## Documentation

- **[Cerebras AI Rate Limiting Guide](docs/CEREBRAS_RATE_LIMITING.md)** - Comprehensive Cerebras configuration and usage
- **[Load Testing Guide](docs/LOAD_TESTING.md)** - Load testing framework documentation and best practices

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