# Configuration Guide

Complete configuration options and examples for Cooldown Proxy.

## Configuration File

Cooldown Proxy uses a YAML configuration file (`config.yaml`) to define server settings and rate limiting rules.

### Basic Configuration Structure

```yaml
server:
  host: "localhost"    # Server bind address
  port: 8080          # Server port

rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10
  - domain: "*.example.com"    # Wildcard support
    requests_per_second: 20

# Optional: Default rate limit for unspecified domains
default_rate_limit:
  requests_per_second: 1
```

## Configuration Options

### Server Settings

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `server.host` | string | "localhost" | Server bind address |
| `server.port` | int | 8080 | Server port |

### Standard Rate Limiting

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `rate_limits` | array | [] | Array of rate limit rules |
| `rate_limits[].domain` | string | - | Domain pattern (supports wildcards) |
| `rate_limits[].requests_per_second` | int | - | Max requests per second |
| `default_rate_limit.requests_per_second` | int | 1 | Default rate for unknown domains |

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

### Example 3: Production with Wildcards

```yaml
server:
  host: "0.0.0.0"
  port: 80

rate_limits:
  # External APIs
  - domain: "api.github.com"
    requests_per_second: 30
  - domain: "api.twitter.com"
    requests_per_second: 15
  - domain: "api.stripe.com"
    requests_per_second: 100

  # Internal services with wildcard support
  - domain: "*.internal.company.com"
    requests_per_second: 500
  - domain: "*.services.company.com"
    requests_per_second: 200

default_rate_limit:
  requests_per_second: 10
```

## Wildcard Domain Matching

The proxy supports wildcard patterns for domain matching:

- `*.example.com` matches `api.example.com`, `app.example.com`, etc.
- `*.api.example.com` matches `v1.api.example.com`, `v2.api.example.com`, etc.
- Wildcards only work at the beginning of the domain pattern

## Rate Limiting Algorithm

Cooldown Proxy uses the **leaky bucket algorithm** for rate limiting:

- **Smooth rate limiting**: Requests are evenly distributed over time
- **Burst capacity**: Allows short bursts while maintaining overall rate
- **Per-domain isolation**: Each domain has its own rate limiter
- **Automatic delay calculation**: Requests exceeding the rate limit are delayed appropriately

### Behavior

```yaml
rate_limits:
  - domain: "api.example.com"
    requests_per_second: 10  # Allows ~1 request every 100ms
```

- Requests exceeding the rate limit will be delayed
- The delay is automatically calculated to maintain the target rate
- Multiple domains are rate-limited independently
- Wildcard patterns provide flexible matching

## Command Line Options

```bash
# Run with default config
./cooldown-proxy

# Run with custom config file
./cooldown-proxy -config /path/to/config.yaml

# Development mode (creates config.yaml if missing)
make dev
```

## Configuration Validation

The proxy validates configuration on startup:

- Checks for valid YAML syntax
- Validates port numbers and host addresses
- Ensures rate limits are positive integers
- Verifies domain patterns are properly formatted

## Environment Variables

- `LOG_LEVEL`: Set logging level (debug, info, warn, error)
- `CONFIG_PATH`: Override default config file path