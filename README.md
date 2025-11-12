# Cooldown Proxy

<div align="center">
  <img src="assets/cooldowsteamcr.png" alt="Cooldown Proxy Banner" width="600">
</div>

A local-first reverse proxy with intelligent rate limiting for outgoing REST API requests using the leaky bucket algorithm.

## Features

- **Intelligent Rate Limiting**: Per-domain rate limiting using the leaky bucket algorithm
- **Wildcard Domain Support**: Support for patterns like `*.example.com`
- **Cerebras AI Rate Limiting**: Dual-metric enforcement (RPM + TPM) with intelligent queuing
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

- **[Configuration Guide](docs/CONFIGURATION.md)** - Complete configuration options and examples
- **[Cerebras AI Rate Limiting](docs/CEREBRAS_RATE_LIMITING.md)** - Advanced Cerebras AI rate limiting
- **[Load Testing](docs/LOAD_TESTING.md)** - Performance testing framework usage
- **[Development Guide](docs/DEVELOPMENT.md)** - Architecture, testing, and contributing

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
│   ├── proxy/             # HTTP proxy handler
│   ├── ratelimit/         # Rate limiting implementation
│   └── router/            # Request routing
├── docs/                  # Documentation
└── config.yaml.example    # Example configuration
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.