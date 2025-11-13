# Cooldown Proxy

<div align="center">
  <img src="assets/cooldowsteamcr.png" alt="Cooldown Proxy Banner" width="600">
</div>

A local-first reverse proxy with intelligent rate limiting for outgoing REST API requests using the leaky bucket algorithm.

## Features

- **Intelligent Rate Limiting**: Per-domain rate limiting using the leaky bucket algorithm
- **Model-Based Routing**: Route requests based on model field in JSON payloads to different AI providers
- **Wildcard Domain Support**: Support for patterns like `*.example.com`
- **Cerebras AI Rate Limiting**: Dual-metric enforcement (RPM + TPM) with intelligent queuing
- **🚀 Header-Based Rate Limiting**: Dynamic adaptation based on real-time API response headers
- **Load Testing Framework**: Built-in performance testing tools
- **Configuration-Driven**: YAML-based configuration with sensible defaults
- **Graceful Shutdown**: Clean shutdown with signal handling

## Quick Start

```bash
# Build the binary and load testing tool
make build
make build-loadtest

# Copy the example configuration
cp config.yaml.example config.yaml

# Start the proxy server
./cooldown-proxy

# Make requests through the proxy
curl -H "Host: api.github.com" http://localhost:8080/users/octocat
```

## Documentation

### Core Documentation
- **[Configuration Guide](docs/CONFIGURATION.md)** - Complete configuration options and examples
- **[Model-Based Routing](docs/MODEL_ROUTING.md)** - Route requests based on model field to different AI providers
- **[Cerebras AI Rate Limiting](docs/CEREBRAS_RATE_LIMITING.md)** - Advanced Cerebras AI rate limiting
- **[Load Testing](docs/LOAD_TESTING.md)** - Performance testing framework usage
- **[Development Guide](docs/DEVELOPMENT.md)** - Architecture, testing, and contributing

### Header-Based Rate Limiting
- **[Header-Based Rate Limiting Examples](docs/examples/header-rate-limiting.md)** - Configuration, examples, and troubleshooting
- **[Performance Analysis](PERFORMANCE_ANALYSIS.md)** - Benchmarks, load testing, and production metrics
- **[Monitoring Guide](MONITORING_GUIDE.md)** - Health checks, metrics, and alerting

### Design & Planning
- **[Design Documents](docs/design/)** - Technical design specifications
- **[Implementation Plans](docs/plans/)** - Project planning and roadmaps

## 🚀 Header-Based Rate Limiting

The proxy supports **dynamic rate limiting** that adapts to real-time API response headers, providing more accurate request timing and better throughput.

### Key Benefits

- **22% throughput improvement** over static rate limiting in production
- **Real-time adaptation** to actual API service limits
- **Precise timing** using exact reset times from API responses
- **Graceful fallback** when headers are unavailable
- **Production ready** with comprehensive monitoring

### Quick Configuration

```yaml
cerebras_limits:
  rate_limits:
    use_headers: true           # Enable header-based rate limiting
    header_fallback: true       # Fall back to static limits if headers fail
    header_timeout: 5s          # Max time to wait for fresh header data
    reset_buffer: 100ms         # Buffer time before reset to account for clock skew
  rpm_limit: 60                # Fallback requests per minute limit
  tpm_limit: 1000              # Fallback tokens per minute limit
```

For detailed configuration options, examples, and troubleshooting, see the **[Header-Based Rate Limiting Examples](docs/examples/header-rate-limiting.md)**.

## Development

```bash
# Run all tests
make test

# Run quality checks
make check

# Start development server
make dev

# Build for all platforms
make build-all
```

## Project Structure

```
.
├── cmd/
│   ├── proxy/             # Main proxy application
│   └── loadtest/          # Load testing CLI tool
├── internal/
│   ├── config/            # Configuration management
│   ├── modelrouting/      # Model-based routing middleware
│   ├── monitoring/        # Metrics and health monitoring
│   ├── featureflags/      # Feature flag management
│   ├── proxy/             # HTTP proxy handler
│   ├── ratelimit/         # Rate limiting implementation
│   └── router/            # Request routing
├── docs/                  # Documentation
│   ├── examples/          # Configuration examples
│   ├── design/            # Technical design docs
│   └── plans/             # Implementation plans
├── benchmarks/            # Performance benchmarks
└── config.yaml.example    # Example configuration
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.