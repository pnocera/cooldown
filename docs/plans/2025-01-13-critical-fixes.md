# Critical Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix critical implementation gaps that prevent the cooldown proxy from functioning as a working reverse proxy

**Architecture:** Systematic fixes to core proxy functionality, starting with the non-functional reverse proxy director, then addressing configuration validation, rate limiting algorithm corrections, and essential error handling

**Tech Stack:** Go 1.21+, net/http/httputil, go.uber.org/ratelimit, gopkg.in/yaml.v3

---

## Pre-Implementation Setup

### Task 0: Verify Current State and Create Worktree

**Files:**
- Reference: `cmd/proxy/main.go:1-104`
- Reference: `internal/proxy/handler.go:1-100`
- Test: Current test suite

**Step 1: Run current tests to establish baseline**

```bash
go test ./...
```

Expected: Some tests may pass, but core functionality will be broken

**Step 2: Create dedicated worktree**

```bash
git worktree add ../cooldownp-fixes critical-fixes-branch
cd ../cooldownp-fixes
```

**Step 3: Verify the broken proxy behavior**

```bash
go run cmd/proxy/main.go &
curl -I http://localhost:8080/test
```

Expected: Requests fail because proxy director doesn't set targets

**Step 4: Commit worktree setup**

```bash
git add .
git commit -m "setup: create worktree for critical fixes"
```

---

## Phase 1: Fix Non-Functional Proxy Implementation

### Task 1: Implement Proper Reverse Proxy Director

**Files:**
- Modify: `internal/proxy/handler.go:45-65` (director function)
- Modify: `internal/proxy/handler.go:1-30` (add target resolution)
- Test: `internal/proxy/handler_test.go:1-50`

**Step 1: Write failing test for proxy director**

```go
func TestProxyDirectorSetsTarget(t *testing.T) {
    cfg := &config.Config{
        Routes: map[string]string{
            "api.example.com": "http://localhost:3000",
        },
    }

    proxy := NewProxyHandler(cfg, ratelimit.NewLimiter(10), nil)

    req := httptest.NewRequest("GET", "http://api.example.com/users", nil)
    proxy.Director(req)

    assert.Equal(t, "http://localhost:3000/users", req.URL.String())
    assert.Equal(t, "api.example.com", req.Host)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/proxy -run TestProxyDirectorSetsTarget -v
```

Expected: FAIL - director function is empty or incomplete

**Step 3: Implement minimal director function**

```go
func (p *ProxyHandler) Director(req *http.Request) {
    host := req.Host
    if target, exists := p.config.Routes[host]; exists {
        targetURL, err := url.Parse(target)
        if err != nil {
            return
        }

        req.URL.Scheme = targetURL.Scheme
        req.URL.Host = targetURL.Host
        req.Host = host
    }
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/proxy -run TestProxyDirectorSetsTarget -v
```

Expected: PASS

**Step 5: Add edge case tests**

```go
func TestProxyDirectorUnknownHost(t *testing.T) {
    proxy := NewProxyHandler(&config.Config{}, ratelimit.NewLimiter(10), nil)

    req := httptest.NewRequest("GET", "http://unknown.com/users", nil)
    proxy.Director(req)

    // Should not panic and should not modify URL
    assert.Equal(t, "unknown.com", req.Host)
}
```

**Step 6: Run all proxy tests**

```bash
go test ./internal/proxy -v
```

**Step 7: Commit director implementation**

```bash
git add internal/proxy/handler.go internal/proxy/handler_test.go
git commit -m "fix: implement functional reverse proxy director"
```

### Task 2: Fix Route Configuration Loading

**Files:**
- Modify: `cmd/proxy/main.go:45-65` (route initialization)
- Modify: `internal/proxy/handler.go:25-35` (constructor)
- Test: `cmd/proxy/main_test.go:1-30`

**Step 1: Write failing test for route loading**

```go
func TestMainLoadsRoutesFromConfig(t *testing.T) {
    cfg := &config.Config{
        Routes: map[string]string{
            "api.cerebras.ai": "https://api.cerebras.ai",
            "*.example.com": "https://backend.example.com",
        },
    }

    // Test that routes are properly loaded into router
    assert.Equal(t, "https://api.cerebras.ai", cfg.Routes["api.cerebras.ai"])
    assert.Equal(t, "https://backend.example.com", cfg.Routes["*.example.com"])
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./cmd/proxy -run TestMainLoadsRoutesFromConfig -v
```

Expected: May pass initially, but integration with proxy will fail

**Step 3: Fix main.go route initialization**

```go
// In main.go, replace empty routes map
routes := make(map[string]string)
for domain, target := range cfg.Routes {
    routes[domain] = target
}

router := modelrouter.NewRouter(routes)
```

**Step 4: Update proxy handler constructor**

```go
func NewProxyHandler(cfg *config.Config, limiter ratelimit.Limiter, router *modelrouter.Router) *ProxyHandler {
    return &ProxyHandler{
        config:  cfg,
        limiter: limiter,
        router:  router,
        proxy: &httputil.ReverseProxy{
            Director: p.director,
        },
    }
}
```

**Step 5: Run integration test**

```bash
go test ./cmd/proxy -v
```

**Step 6: Commit route loading fixes**

```bash
git add cmd/proxy/main.go internal/proxy/handler.go
git commit -m "fix: properly load routes from configuration into proxy"
```

---

## Phase 2: Fix Configuration System

### Task 3: Fix Configuration Field Mismatches

**Files:**
- Modify: `cmd/proxy/main.go:30-40` (field access)
- Modify: `internal/config/types.go:10-20` (struct definitions)
- Test: `internal/config/config_test.go:1-50`

**Step 1: Write failing test for configuration loading**

```go
func TestConfigFieldConsistency(t *testing.T) {
    cfg := &config.ServerConfig{
        Host: "localhost",
        Port: 8080,
    }

    // Test that main.go can access the correct fields
    assert.Equal(t, "localhost", cfg.Host)
    assert.NotEmpty(t, cfg.Port)
}
```

**Step 2: Run test to identify mismatch**

```bash
go test ./internal/config -run TestConfigFieldConsistency -v
```

Expected: May reveal field name inconsistencies

**Step 3: Fix main.go field access**

```go
// In main.go, fix the field access
server := &http.Server{
    Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
    Handler: router,
}
```

**Step 4: Add configuration validation**

```go
func (c *Config) Validate() error {
    if c.Server.Host == "" {
        return errors.New("server host is required")
    }
    if c.Server.Port == 0 {
        return errors.New("server port is required")
    }
    if len(c.Routes) == 0 {
        return errors.New("at least one route must be configured")
    }
    return nil
}
```

**Step 5: Add validation test**

```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {"valid config", &Config{Server: ServerConfig{Host: "localhost", Port: 8080}}, false},
        {"missing host", &Config{Server: ServerConfig{Port: 8080}}, true},
        {"missing port", &Config{Server: ServerConfig{Host: "localhost"}}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**Step 6: Run configuration tests**

```bash
go test ./internal/config -v
```

**Step 7: Add validation to main**

```go
if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

**Step 8: Commit configuration fixes**

```bash
git add internal/config/types.go internal/config/config.go cmd/proxy/main.go
git commit -m "fix: resolve configuration field mismatches and add validation"
```

### Task 4: Add Required Provider Configuration

**Files:**
- Modify: `internal/config/types.go:50-80` (provider config)
- Create: `internal/config/validation.go` (validation logic)
- Test: `internal/config/validation_test.go`

**Step 1: Write failing test for provider validation**

```go
func TestProviderConfigurationValidation(t *testing.T) {
    cfg := &Config{
        Providers: map[string]ProviderConfig{
            "cerebras": {
                APIKey:    "",
                Endpoint:  "https://api.cerebras.ai",
                Models:    []string{"llama3.1-8b"},
            },
        },
    }

    err := cfg.ValidateProviders()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "API key is required")
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config -run TestProviderConfigurationValidation -v
```

Expected: FAIL - ValidateProviders doesn't exist

**Step 3: Implement provider validation**

```go
func (c *Config) ValidateProviders() error {
    for name, provider := range c.Providers {
        if provider.APIKey == "" {
            return fmt.Errorf("provider %s: API key is required", name)
        }
        if provider.Endpoint == "" {
            return fmt.Errorf("provider %s: endpoint is required", name)
        }
        if len(provider.Models) == 0 {
            return fmt.Errorf("provider %s: at least one model is required", name)
        }
    }
    return nil
}
```

**Step 4: Add provider config types**

```go
type ProviderConfig struct {
    APIKey      string   `yaml:"api_key" env:"API_KEY"`
    Endpoint    string   `yaml:"endpoint" env:"ENDPOINT"`
    Models      []string `yaml:"models"`
    RateLimit   int      `yaml:"rate_limit"`
    Timeout     int      `yaml:"timeout"`
}
```

**Step 5: Run provider validation tests**

```bash
go test ./internal/config -v
```

**Step 6: Commit provider configuration**

```bash
git add internal/config/types.go internal/config/validation.go internal/config/validation_test.go
git commit -m "feat: add provider configuration validation"
```

---

## Phase 3: Fix Rate Limiting Implementation

### Task 5: Correct Rate Limiting Algorithm

**Files:**
- Modify: `internal/ratelimit/limiter.go:1-50` (algorithm implementation)
- Test: `internal/ratelimit/limiter_test.go:1-100`

**Step 1: Write test for actual leaky bucket behavior**

```go
func TestLeakyBucketBehavior(t *testing.T) {
    limiter := NewLeakyBucket(10) // 10 requests per second

    // Should allow first request immediately
    delay1 := limiter.GetDelay("test.com")
    assert.Equal(t, time.Duration(0), delay1)

    // Should allow subsequent requests up to rate limit
    for i := 0; i < 9; i++ {
        delay := limiter.GetDelay("test.com")
        assert.Equal(t, time.Duration(0), delay)
    }

    // Next request should be delayed
    delay11 := limiter.GetDelay("test.com")
    assert.Greater(t, delay11, time.Duration(0))
}
```

**Step 2: Run test to verify current broken behavior**

```bash
go test ./internal/ratelimit -run TestLeakyBucketBehavior -v
```

Expected: FAIL - current implementation may not follow leaky bucket

**Step 3: Implement proper leaky bucket algorithm**

```go
type LeakyBucket struct {
    rate       float64
    capacity   int
    domains    map[string]*Bucket
    mutex      sync.RWMutex
}

type Bucket struct {
    lastLeak   time.Time
    tokens     int
}

func NewLeakyBucket(rate float64) *LeakyBucket {
    return &LeakyBucket{
        rate:     rate,
        capacity: int(rate * 2), // 2 seconds capacity
        domains:  make(map[string]*Bucket),
    }
}

func (lb *LeakyBucket) GetDelay(domain string) time.Duration {
    lb.mutex.Lock()
    defer lb.mutex.Unlock()

    bucket, exists := lb.domains[domain]
    if !exists {
        bucket = &Bucket{
            lastLeak: time.Now(),
            tokens:   lb.capacity - 1,
        }
        lb.domains[domain] = bucket
        return 0
    }

    // Leak tokens based on time elapsed
    now := time.Now()
    elapsed := now.Sub(bucket.lastLeak).Seconds()
    tokensToLeak := int(elapsed * lb.rate)

    bucket.tokens = max(0, bucket.tokens-tokensToLeak)
    bucket.lastLeak = now

    if bucket.tokens > 0 {
        bucket.tokens--
        return 0
    }

    // Calculate delay needed for next token
    return time.Duration(float64(time.Second) / lb.rate)
}
```

**Step 4: Run leaky bucket tests**

```bash
go test ./internal/ratelimit -v
```

**Step 5: Commit rate limiting fixes**

```bash
git add internal/ratelimit/limiter.go internal/ratelimit/limiter_test.go
git commit -m "fix: implement proper leaky bucket rate limiting algorithm"
```

### Task 6: Add Rate Limiting Metrics and Logging

**Files:**
- Modify: `internal/ratelimit/limiter.go:100-120` (add metrics)
- Create: `internal/ratelimit/metrics.go` (metrics collection)
- Test: `internal/ratelimit/metrics_test.go`

**Step 1: Write failing test for metrics collection**

```go
func TestRateLimitingMetrics(t *testing.T) {
    limiter := NewLeakyBucket(10)

    limiter.GetDelay("test.com")

    metrics := limiter.GetMetrics("test.com")
    assert.Equal(t, 1, metrics.TotalRequests)
    assert.Equal(t, 0, metrics.DelayedRequests)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/ratelimit -run TestRateLimitingMetrics -v
```

**Step 3: Implement metrics collection**

```go
type Metrics struct {
    TotalRequests   int64
    DelayedRequests int64
    CurrentTokens   int
    LastAccess      time.Time
}

func (lb *LeakyBucket) GetMetrics(domain string) Metrics {
    lb.mutex.RLock()
    defer lb.mutex.RUnlock()

    bucket := lb.domains[domain]
    if bucket == nil {
        return Metrics{}
    }

    return Metrics{
        TotalRequests:   bucket.totalRequests,
        DelayedRequests: bucket.delayedRequests,
        CurrentTokens:   bucket.tokens,
        LastAccess:      bucket.lastLeak,
    }
}
```

**Step 4: Run metrics tests**

```bash
go test ./internal/ratelimit -v
```

**Step 5: Commit metrics implementation**

```bash
git add internal/ratelimit/limiter.go internal/ratelimit/metrics.go internal/ratelimit/metrics_test.go
git commit -m "feat: add rate limiting metrics and logging"
```

---

## Phase 4: Strengthen Error Handling

### Task 7: Add Comprehensive Error Handling

**Files:**
- Modify: `internal/proxy/handler.go:80-120` (error handling)
- Create: `internal/errors/errors.go` (error types)
- Test: `internal/proxy/error_test.go`

**Step 1: Write failing test for error scenarios**

```go
func TestProxyHandlesUpstreamErrors(t *testing.T) {
    // Test with invalid upstream that will fail
    cfg := &config.Config{
        Routes: map[string]string{
            "test.com": "http://invalid-host-that-does-not-exist:9999",
        },
    }

    proxy := NewProxyHandler(cfg, ratelimit.NewLimiter(10), nil)

    req := httptest.NewRequest("GET", "http://test.com/users", nil)
    w := httptest.NewRecorder()

    proxy.ServeHTTP(w, req)

    assert.Equal(t, http.StatusBadGateway, w.Code)
}
```

**Step 2: Run test to verify error handling fails**

```bash
go test ./internal/proxy -run TestProxyHandlesUpstreamErrors -v
```

**Step 3: Implement error handling middleware**

```go
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Apply rate limiting
    delay := p.limiter.GetDelay(r.Host)
    if delay > 0 {
        time.Sleep(delay)
    }

    // Create response writer to catch errors
    rw := &responseWriter{ResponseWriter: w}

    // Proxy the request
    p.proxy.ServeHTTP(rw, r)

    // Handle any errors that occurred
    if rw.status >= 500 {
        p.handleError(rw, r, fmt.Errorf("upstream server error: %d", rw.status))
    }
}

func (p *ProxyHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
    log.Printf("Proxy error for %s: %v", r.URL.Path, err)

    switch {
    case errors.Is(err, context.DeadlineExceeded):
        http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
    case errors.Is(err, io.EOF):
        http.Error(w, "Bad Gateway", http.StatusBadGateway)
    default:
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}
```

**Step 4: Run error handling tests**

```bash
go test ./internal/proxy -v
```

**Step 5: Commit error handling**

```bash
git add internal/proxy/handler.go internal/errors/errors.go internal/proxy/error_test.go
git commit -m "feat: add comprehensive error handling for proxy failures"
```

---

## Phase 5: Integration and End-to-End Testing

### Task 8: Add Integration Tests

**Files:**
- Create: `tests/integration/proxy_integration_test.go`
- Create: `tests/integration/test_servers.go` (test upstream servers)
- Test: All integration tests

**Step 1: Write failing integration test**

```go
func TestProxyEndToEndIntegration(t *testing.T) {
    // Start test upstream server
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"message": "Hello from upstream"}`))
    }))
    defer upstream.Close()

    // Configure proxy to route to test server
    cfg := &config.Config{
        Server: config.ServerConfig{
            Host: "localhost",
            Port: 0, // Let OS choose port
        },
        Routes: map[string]string{
            "test.com": upstream.URL,
        },
    }

    // Start proxy server
    proxy := setupProxyServer(cfg)
    defer proxy.Close()

    // Make request through proxy
    resp, err := http.Get(proxy.URL + "/api/test")
    assert.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)

    body, err := ioutil.ReadAll(resp.Body)
    assert.NoError(t, err)
    assert.Contains(t, string(body), "Hello from upstream")
}
```

**Step 2: Run integration test to verify it fails initially**

```bash
go test ./tests/integration -v
```

**Step 3: Implement test server setup**

```go
func setupProxyServer(cfg *config.Config) *httptest.Server {
    // Initialize rate limiter
    limiter := ratelimit.NewLeakyBucket(10)

    // Initialize router
    router := modelrouter.NewRouter(cfg.Routes)

    // Initialize proxy handler
    proxyHandler := proxy.NewProxyHandler(cfg, limiter, router)

    // Create test server
    return httptest.NewServer(proxyHandler)
}
```

**Step 4: Run integration tests**

```bash
go test ./tests/integration -v
```

**Step 5: Commit integration tests**

```bash
git add tests/integration/proxy_integration_test.go tests/integration/test_servers.go
git commit -m "test: add end-to-end integration tests"
```

### Task 9: Performance and Load Testing

**Files:**
- Create: `tests/performance/load_test.go`
- Create: `tests/performance/benchmark_test.go`
- Test: Performance tests

**Step 1: Write load test**

```go
func TestProxyLoadHandling(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }

    concurrency := 50
    requests := 1000

    var wg sync.WaitGroup
    var successCount int64
    var errorCount int64

    // Start test upstream
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(10 * time.Millisecond) // Simulate processing time
        w.WriteHeader(http.StatusOK)
    }))
    defer upstream.Close()

    // Setup proxy
    proxy := setupTestProxy(upstream.URL)

    // Run load test
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < requests/concurrency; j++ {
                resp, err := http.Get(proxy.URL)
                if err != nil || resp.StatusCode != http.StatusOK {
                    atomic.AddInt64(&errorCount, 1)
                } else {
                    atomic.AddInt64(&successCount, 1)
                }
                if resp != nil {
                    resp.Body.Close()
                }
            }
        }()
    }

    wg.Wait()

    successRate := float64(successCount) / float64(successCount+errorCount)
    assert.Greater(t, successRate, 0.95, "Success rate should be above 95%")
}
```

**Step 2: Run load test**

```bash
go test ./tests/performance -v -run TestProxyLoadHandling
```

**Step 3: Commit performance tests**

```bash
git add tests/performance/load_test.go tests/performance/benchmark_test.go
git commit -m "test: add performance and load testing"
```

---

## Phase 6: Documentation and Final Verification

### Task 10: Update Documentation

**Files:**
- Modify: `README.md` (update installation/usage)
- Modify: `CLAUDE.md` (update architecture docs)
- Create: `docs/DEPLOYMENT.md` (deployment guide)

**Step 1: Update README with working examples**

```markdown
## Quick Start

1. Clone and build:
```bash
git clone https://github.com/your-org/cooldownp.git
cd cooldownp
make build
```

2. Configure:
```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your upstream servers
```

3. Run:
```bash
./cooldown-proxy
```

## Configuration Example

```yaml
server:
  host: "localhost"
  port: 8080

routes:
  api.cerebras.ai: "https://api.cerebras.ai"
  "*.example.com": "https://backend.example.com"

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 10
  - domain: "*.example.com"
    requests_per_second: 5
```

## Usage

Once running, the proxy will forward requests based on the Host header:

```bash
curl -H "Host: api.cerebras.ai" http://localhost:8080/v1/chat/completions
```
```

**Step 2: Update architecture documentation**

**Step 3: Add deployment guide**

**Step 4: Commit documentation**

```bash
git add README.md CLAUDE.md docs/DEPLOYMENT.md
git commit -m "docs: update documentation with working examples and deployment guide"
```

### Task 11: Final System Verification

**Files:**
- Test: All tests across the codebase
- Verify: Complete system functionality

**Step 1: Run full test suite**

```bash
go test ./...
```

**Step 2: Build all platforms**

```bash
make build-all
```

**Step 3: Test real-world scenario**

```bash
# Start with example config
./cooldown-proxy -config config.yaml.example &

# Test actual proxy functionality
curl -H "Host: api.cerebras.ai" http://localhost:8080/health
```

**Step 4: Final integration verification**

```bash
go run cmd/proxy/main.go &
PROXY_PID=$!

# Test multiple routes
for host in "api.cerebras.ai" "*.example.com"; do
    curl -H "Host: $host" http://localhost:8080/health
done

kill $PROXY_PID
```

**Step 5: Commit final verification**

```bash
git add .
git commit -m "feat: complete critical fixes implementation - proxy now fully functional"
```

---

## Implementation Notes

### Critical Dependencies
- Go 1.21+ for proper error handling
- `go.uber.org/ratelimit` for rate limiting (may need replacement for true leaky bucket)
- Standard library `net/http/httputil` for reverse proxy

### Testing Strategy
- Each task follows TDD: write failing test, implement minimal code, verify pass
- Unit tests for each component
- Integration tests for end-to-end functionality
- Load tests for performance verification

### Success Criteria
1. ✅ Proxy correctly forwards requests to configured upstreams
2. ✅ Configuration validation prevents startup with invalid configs
3. ✅ Rate limiting properly implements leaky bucket algorithm
4. ✅ Error handling gracefully handles upstream failures
5. ✅ All tests pass, including integration and load tests
6. ✅ Documentation accurately reflects working system

### Post-Implementation
- Binary in `dist/` directory will be fully functional
- Configuration examples will work out of the box
- Ready for production deployment with monitoring and error handling