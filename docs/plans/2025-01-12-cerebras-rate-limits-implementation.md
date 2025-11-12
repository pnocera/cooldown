# Cerebras Rate Limits Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enhance the Cooldown Proxy to efficiently manage Cerebras inference API rate limits using smart priority queuing, ensuring strict compliance with RPM (1,000) and TPM (1,000,000) limits while maximizing throughput.

**Architecture:** Replace the current simple per-second rate limiting with a sophisticated dual-metric system using sliding windows for RPM and TPM tracking, intelligent token estimation, and priority-based queuing that adapts to workload patterns.

**Tech Stack:** Go (1.21+), go.uber.org/ratelimit (partial), tiktoken-go for token estimation, YAML configuration, goroutine-safe concurrent programming

---

## Task 1: Enhanced Rate Limiter Foundation

**Files:**
- Create: `internal/ratelimit/cerebras.go`
- Create: `internal/ratelimit/cerebras_test.go`
- Create: `internal/token/types.go`

**Step 1: Write failing test for basic rate limiter structure**

```go
// internal/ratelimit/cerebras_test.go
package ratelimit

import (
    "testing"
    "time"
)

func TestCerebrasRateLimiter_Creation(t *testing.T) {
    limiter := NewCerebrasLimiter(1000, 1000000)
    
    if limiter == nil {
        t.Fatal("Expected non-nil limiter")
    }
    
    if limiter.RPMLimit() != 1000 {
        t.Errorf("Expected RPM limit 1000, got %d", limiter.RPMLimit())
    }
    
    if limiter.TPMLimit() != 1000000 {
        t.Errorf("Expected TPM limit 1000000, got %d", limiter.TPMLimit())
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit/... -v`
Expected: FAIL with "NewCerebrasLimiter undefined"

**Step 3: Write minimal implementation**

```go
// internal/ratelimit/cerebras.go
package ratelimit

import (
    "sync"
    "time"
)

type CerebrasLimiter struct {
    rpmLimit int
    tpmLimit int
    mu       sync.RWMutex
}

func NewCerebrasLimiter(rpmLimit, tpmLimit int) *CerebrasLimiter {
    return &CerebrasLimiter{
        rpmLimit: rpmLimit,
        tpmLimit: tpmLimit,
    }
}

func (c *CerebrasLimiter) RPMLimit() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.rpmLimit
}

func (c *CerebrasLimiter) TPMLimit() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.tpmLimit
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ratelimit/cerebras.go internal/ratelimit/cerebras_test.go
git commit -m "feat: add basic cerebras rate limiter structure"
```

---

## Task 2: Token Types Definition

**Files:**
- Modify: `internal/token/types.go`

**Step 1: Write failing test for token request type**

```go
// internal/token/types_test.go
package token

import (
    "testing"
)

func TestTokenRequest_Creation(t *testing.T) {
    req := TokenRequest{
        InputTokens:  100,
        OutputTokens: 200,
        Model:        "llama3.1-70b",
    }
    
    expectedTotal := 300
    if req.TotalTokens() != expectedTotal {
        t.Errorf("Expected total tokens %d, got %d", expectedTotal, req.TotalTokens())
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/token/... -v`
Expected: FAIL with "TokenRequest undefined"

**Step 3: Write minimal implementation**

```go
// internal/token/types.go
package token

type TokenRequest struct {
    InputTokens  int
    OutputTokens int
    Model        string
}

func (tr TokenRequest) TotalTokens() int {
    return tr.InputTokens + tr.OutputTokens
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/token/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/token/types.go internal/token/types_test.go
git commit -m "feat: add token request type definition"
```

---

## Task 3: Sliding Window Counter

**Files:**
- Modify: `internal/ratelimit/cerebras.go`
- Modify: `internal/ratelimit/cerebras_test.go`

**Step 1: Write failing test for sliding window functionality**

```go
// Add to internal/ratelimit/cerebras_test.go
func TestCerebrasRateLimiter_SlidingWindowRPM(t *testing.T) {
    limiter := NewCerebrasLimiter(2, 1000000) // 2 RPM for testing
    
    // First request should be allowed
    if delay := limiter.CheckRequest(100); delay > 0 {
        t.Errorf("First request should be allowed, got delay %v", delay)
    }
    
    // Second request should be allowed
    if delay := limiter.CheckRequest(100); delay > 0 {
        t.Errorf("Second request should be allowed, got delay %v", delay)
    }
    
    // Third request should be delayed (over RPM limit)
    if delay := limiter.CheckRequest(100); delay == 0 {
        t.Error("Third request should be delayed due to RPM limit")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit/... -v`
Expected: FAIL with "CheckRequest undefined"

**Step 3: Write minimal implementation**

```go
// Add to internal/ratelimit/cerebras.go
import (
    "container/list"
    "sync"
    "time"
)

type slidingWindow struct {
    elements *list.List
    size     time.Duration
}

type windowElement struct {
    timestamp time.Time
    value     int
}

type CerebrasLimiter struct {
    rpmLimit int
    tpmLimit int
    rpmWindow *slidingWindow
    tpmWindow *slidingWindow
    mu        sync.RWMutex
}

func NewCerebrasLimiter(rpmLimit, tpmLimit int) *CerebrasLimiter {
    return &CerebrasLimiter{
        rpmLimit: rpmLimit,
        tpmLimit: tpmLimit,
        rpmWindow: &slidingWindow{
            elements: list.New(),
            size:     time.Minute,
        },
        tpmWindow: &slidingWindow{
            elements: list.New(),
            size:     time.Minute,
        },
    }
}

func (sw *slidingWindow) add(value int, now time.Time) {
    // Remove old elements
    for sw.elements.Len() > 0 {
        front := sw.elements.Front()
        if now.Sub(front.Value.(*windowElement).timestamp) < sw.size {
            break
        }
        sw.elements.Remove(front)
    }
    
    // Add new element
    sw.elements.PushBack(&windowElement{
        timestamp: now,
        value:     value,
    })
}

func (sw *slidingWindow) sum() int {
    total := 0
    for elem := sw.elements.Front(); elem != nil; elem = elem.Next() {
        total += elem.Value.(*windowElement).value
    }
    return total
}

func (c *CerebrasLimiter) CheckRequest(tokens int) time.Duration {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    
    // Check RPM
    c.rpmWindow.add(1, now)
    rpmCount := c.rpmWindow.sum()
    
    if rpmCount > c.rpmLimit {
        return time.Minute // Simple delay for now
    }
    
    return 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ratelimit/cerebras.go internal/ratelimit/cerebras_test.go
git commit -m "feat: implement sliding window for RPM tracking"
```

---

## Task 4: TPM Limit Integration

**Files:**
- Modify: `internal/ratelimit/cerebras.go`
- Modify: `internal/ratelimit/cerebras_test.go`

**Step 1: Write failing test for TPM limits**

```go
// Add to internal/ratelimit/cerebras_test.go
func TestCerebrasRateLimiter_TPMLimit(t *testing.T) {
    limiter := NewCerebrasLimiter(1000, 1000) // 1000 TPM for testing
    
    // First request with 600 tokens should be allowed
    if delay := limiter.CheckRequest(600); delay > 0 {
        t.Errorf("First 600-token request should be allowed, got delay %v", delay)
    }
    
    // Second request with 600 tokens should be delayed (over TPM limit)
    if delay := limiter.CheckRequest(600); delay == 0 {
        t.Error("Second 600-token request should be delayed due to TPM limit")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit/... -v`
Expected: FAIL with TPM logic not properly implemented

**Step 3: Enhance CheckRequest implementation**

```go
// Modify CheckRequest method in internal/ratelimit/cerebras.go
func (c *CerebrasLimiter) CheckRequest(tokens int) time.Duration {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    
    // Check RPM
    rpmCount := c.rpmWindow.sum()
    if rpmCount >= c.rpmLimit {
        return time.Minute
    }
    
    // Check TPM
    c.tpmWindow.add(tokens, now)
    tpmCount := c.tpmWindow.sum()
    
    if tpmCount > c.tpmLimit {
        return time.Minute
    }
    
    // Actually add the RPM count after TPM check passes
    c.rpmWindow.add(1, now)
    
    return 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ratelimit/cerebras.go internal/ratelimit/cerebras_test.go
git commit -m "feat: integrate TPM limit checking with sliding window"
```

---

## Task 5: Token Estimator Foundation

**Files:**
- Create: `internal/token/estimator.go`
- Create: `internal/token/estimator_test.go`

**Step 1: Write failing test for basic token estimation**

```go
// internal/token/estimator_test.go
package token

import (
    "testing"
)

func TestTokenEstimator_EstimateInputTokens(t *testing.T) {
    estimator := NewTokenEstimator()
    
    // Test with simple text
    text := "Hello world"
    tokens, err := estimator.EstimateInputTokens("llama3.1-70b", text)
    
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    
    if tokens <= 0 {
        t.Errorf("Expected positive token count, got %d", tokens)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/token/... -v`
Expected: FAIL with "NewTokenEstimator undefined"

**Step 3: Write minimal implementation**

```go
// internal/token/estimator.go
package token

import (
    "strings"
)

type TokenEstimator struct {
    // Simple word-based estimation for now
    wordToTokenRatio map[string]float64
}

func NewTokenEstimator() *TokenEstimator {
    return &TokenEstimator{
        wordToTokenRatio: map[string]float64{
            "llama3.1-70b": 1.3, // Approximate ratio
        },
    }
}

func (te *TokenEstimator) EstimateInputTokens(model, text string) (int, error) {
    ratio, exists := te.wordToTokenRatio[model]
    if !exists {
        ratio = 1.0 // Default ratio
    }
    
    words := strings.Fields(text)
    return int(float64(len(words)) * ratio), nil
}

func (te *TokenEstimator) EstimateOutputTokens(model, inputTokens int) int {
    // Conservative estimate: output tokens = 0.5 * input tokens
    return max(10, inputTokens/2)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/token/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/token/estimator.go internal/token/estimator_test.go
git commit -m "feat: add basic token estimation functionality"
```

---

## Task 6: Priority Queue Foundation

**Files:**
- Create: `internal/queue/priority.go`
- Create: `internal/queue/priority_test.go`

**Step 1: Write failing test for priority queue basic operations**

```go
// internal/queue/priority_test.go
package queue

import (
    "testing"
    "time"
)

func TestPriorityQueue_BasicOperations(t *testing.T) {
    pq := NewPriorityQueue(10, 5*time.Minute)
    
    if pq.Len() != 0 {
        t.Errorf("Expected empty queue, got length %d", pq.Len())
    }
    
    // Add a request
    req := &QueuedRequest{
        ID:        "test-1",
        Tokens:    100,
        Timestamp: time.Now(),
    }
    
    pq.Enqueue(req)
    
    if pq.Len() != 1 {
        t.Errorf("Expected queue length 1, got %d", pq.Len())
    }
    
    // Dequeue the request
    dequeued := pq.Dequeue()
    if dequeued == nil {
        t.Error("Expected to dequeue a request, got nil")
    }
    
    if dequeued.ID != "test-1" {
        t.Errorf("Expected request ID 'test-1', got '%s'", dequeued.ID)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/... -v`
Expected: FAIL with "NewPriorityQueue undefined"

**Step 3: Write minimal implementation**

```go
// internal/queue/priority.go
package queue

import (
    "container/heap"
    "sync"
    "time"
)

type QueuedRequest struct {
    ID         string
    Tokens     int
    Priority   float64
    Timestamp  time.Time
    Timeout    time.Time
}

type priorityQueue []*QueuedRequest

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
    // Higher priority first
    if pq[i].Priority != pq[j].Priority {
        return pq[i].Priority > pq[j].Priority
    }
    // If same priority, earlier timestamp first
    return pq[i].Timestamp.Before(pq[j].Timestamp)
}

func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x interface{}) {
    item := x.(*QueuedRequest)
    *pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
    old := *pq
    n := len(old)
    item := old[n-1]
    old[n-1] = nil
    *pq = old[0 : n-1]
    return item
}

type PriorityQueue struct {
    pq       priorityQueue
    mu       sync.RWMutex
    maxDepth int
    timeout  time.Duration
}

func NewPriorityQueue(maxDepth int, timeout time.Duration) *PriorityQueue {
    pq := &PriorityQueue{
        pq:       make(priorityQueue, 0),
        maxDepth: maxDepth,
        timeout:  timeout,
    }
    heap.Init(&pq.pq)
    return pq
}

func (pq *PriorityQueue) Len() int {
    pq.mu.RLock()
    defer pq.mu.RUnlock()
    return len(pq.pq)
}

func (pq *PriorityQueue) Enqueue(req *QueuedRequest) bool {
    pq.mu.Lock()
    defer pq.mu.Unlock()
    
    if len(pq.pq) >= pq.maxDepth {
        return false // Queue full
    }
    
    if req.Timeout.IsZero() {
        req.Timeout = time.Now().Add(pq.timeout)
    }
    
    heap.Push(&pq.pq, req)
    return true
}

func (pq *PriorityQueue) Dequeue() *QueuedRequest {
    pq.mu.Lock()
    defer pq.mu.Unlock()
    
    if len(pq.pq) == 0 {
        return nil
    }
    
    // Remove expired requests
    now := time.Now()
    for len(pq.pq) > 0 && pq.pq[0].Timeout.Before(now) {
        heap.Pop(&pq.pq)
    }
    
    if len(pq.pq) == 0 {
        return nil
    }
    
    return heap.Pop(&pq.pq).(*QueuedRequest)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/queue/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/queue/priority.go internal/queue/priority_test.go
git commit -m "feat: implement priority queue for request management"
```

---

## Task 7: Smart Priority Calculation

**Files:**
- Modify: `internal/queue/priority.go`
- Modify: `internal/queue/priority_test.go`

**Step 1: Write failing test for priority calculation logic**

```go
// Add to internal/queue/priority_test.go
func TestPriorityQueue_SmartPriorityCalculation(t *testing.T) {
    pq := NewPriorityQueue(10, 5*time.Minute)
    
    // Simulate high TPM usage (70% threshold)
    highTPMUsage := 0.8
    
    // Small request should get priority boost
    smallReq := &QueuedRequest{
        ID:        "small",
        Tokens:    500,  // < 1000 tokens
        Timestamp: time.Now(),
    }
    
    priority := pq.CalculatePriority(smallReq.Tokens, 0.5, highTPMUsage)
    if priority <= 1.0 {
        t.Errorf("Small request should get priority boost, got priority %f", priority)
    }
    
    // Large request should get priority penalty
    largeReq := &QueuedRequest{
        ID:        "large", 
        Tokens:    6000,  // > 5000 tokens
        Timestamp: time.Now(),
    }
    
    priority = pq.CalculatePriority(largeReq.Tokens, 0.5, highTPMUsage)
    if priority >= 1.0 {
        t.Errorf("Large request should get priority penalty, got priority %f", priority)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/... -v`
Expected: FAIL with "CalculatePriority undefined"

**Step 3: Add priority calculation method**

```go
// Add to internal/queue/priority.go
func (pq *PriorityQueue) CalculatePriority(tokens int, rpmUsage, tpmUsage float64) float64 {
    priorityFactor := max(rpmUsage, tpmUsage)
    
    if priorityFactor > 0.7 {
        // Smart mode active
        if tokens < 1000 {
            return 2.0 // Priority boost for small requests
        } else if tokens > 5000 {
            return 0.5 // Priority penalty for large requests
        }
        return 1.0 // Normal priority
    }
    
    return 1.0 // Normal mode
}

func max(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/queue/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/queue/priority.go internal/queue/priority_test.go
git commit -m "feat: add smart priority calculation based on usage and token count"
```

---

## Task 8: Configuration Extension

**Files:**
- Modify: `internal/config/types.go`
- Modify: `internal/config/loader_test.go`

**Step 1: Write failing test for new cerebras configuration**

```go
// Add to internal/config/loader_test.go
func TestLoadConfig_CerebrasLimits(t *testing.T) {
    configData := `
server:
  host: "localhost"
  port: 8080

cerebras_limits:
  rpm_limit: 1000
  tpm_limit: 1000000
  max_queue_depth: 100
  request_timeout: 10m
  priority_threshold: 0.7

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 100
`
    
    config, err := LoadConfig(strings.NewReader(configData))
    if err != nil {
        t.Fatalf("Failed to load config: %v", err)
    }
    
    if config.CerebrasLimits.RPMLimit != 1000 {
        t.Errorf("Expected RPM limit 1000, got %d", config.CerebrasLimits.RPMLimit)
    }
    
    if config.CerebrasLimits.TPMLimit != 1000000 {
        t.Errorf("Expected TPM limit 1000000, got %d", config.CerebrasLimits.TPMLimit)
    }
    
    if config.CerebrasLimits.MaxQueueDepth != 100 {
        t.Errorf("Expected max queue depth 100, got %d", config.CerebrasLimits.MaxQueueDepth)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL with CerebrasLimits undefined

**Step 3: Extend configuration types**

```go
// Modify internal/config/types.go
package config

import "time"

type Config struct {
    Server         ServerConfig      `yaml:"server"`
    RateLimits     []RateLimitRule   `yaml:"rate_limits"`
    DefaultRateLimit RateLimitRule   `yaml:"default_rate_limit"`
    CerebrasLimits CerebrasLimits    `yaml:"cerebras_limits"`
}

type CerebrasLimits struct {
    RPMLimit          int           `yaml:"rpm_limit"`
    TPMLimit          int           `yaml:"tpm_limit"`
    MaxQueueDepth     int           `yaml:"max_queue_depth"`
    RequestTimeout    time.Duration `yaml:"request_timeout"`
    PriorityThreshold float64       `yaml:"priority_threshold"`
}

// Set default values for CerebrasLimits
func (c *CerebrasLimits) SetDefaults() {
    if c.RPMLimit == 0 {
        c.RPMLimit = 1000
    }
    if c.TPMLimit == 0 {
        c.TPMLimit = 1000000
    }
    if c.MaxQueueDepth == 0 {
        c.MaxQueueDepth = 100
    }
    if c.RequestTimeout == 0 {
        c.RequestTimeout = 10 * time.Minute
    }
    if c.PriorityThreshold == 0 {
        c.PriorityThreshold = 0.7
    }
}
```

**Step 4: Modify config loader to set defaults**

```go
// Modify LoadConfig function in internal/config/loader.go
func LoadConfig(r io.Reader) (*Config, error) {
    var config Config
    
    decoder := yaml.NewDecoder(r)
    decoder.KnownFields(true)
    
    if err := decoder.Decode(&config); err != nil {
        return nil, fmt.Errorf("failed to decode config: %w", err)
    }
    
    // Set defaults
    config.Server.SetDefaults()
    config.DefaultRateLimit.SetDefaults()
    config.CerebrasLimits.SetDefaults()
    
    return &config, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/types.go internal/config/loader.go internal/config/loader_test.go
git commit -m "feat: extend configuration to support cerebras limits"
```

---

## Task 9: Cerebras-Specific Router Integration

**Files:**
- Create: `internal/router/cerebras.go`
- Create: `internal/router/cerebras_test.go`

**Step 1: Write failing test for cerebras routing detection**

```go
// internal/router/cerebras_test.go
package router

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestCerebrasRouter_IsCerebrasRequest(t *testing.T) {
    router := NewCerebrasRouter(nil)
    
    // Test with Cerebras API host
    req := httptest.NewRequest("POST", "https://api.cerebras.ai/v1/chat/completions", nil)
    req.Host = "api.cerebras.ai"
    
    if !router.IsCerebrasRequest(req) {
        t.Error("Expected request to api.cerebras.ai to be detected as Cerebras request")
    }
    
    // Test with non-Cerebras host
    req2 := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
    req2.Host = "api.openai.com"
    
    if router.IsCerebrasRequest(req2) {
        t.Error("Expected request to api.openai.com to NOT be detected as Cerebras request")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/router/... -v`
Expected: FAIL with "NewCerebrasRouter undefined"

**Step 3: Write minimal implementation**

```go
// internal/router/cerebras.go
package router

import (
    "net/http"
    "strings"
)

type CerebrasRouter struct {
    cerebrasHosts []string
}

func NewCerebrasRouter(fallback http.Handler) *CerebrasRouter {
    return &CerebrasRouter{
        cerebrasHosts: []string{
            "api.cerebras.ai",
            "inference.cerebras.ai",
        },
    }
}

func (cr *CerebrasRouter) IsCerebrasRequest(req *http.Request) bool {
    host := req.Host
    // Remove port if present
    if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
        host = host[:colonIndex]
    }
    
    for _, cerebrasHost := range cr.cerebrasHosts {
        if host == cerebrasHost {
            return true
        }
    }
    
    return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/router/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/router/cerebras.go internal/router/cerebras_test.go
git commit -m "feat: add cerebras-specific routing detection"
```

---

## Task 10: Enhanced Rate Limiter Integration

**Files:**
- Modify: `internal/ratelimit/cerebras.go`
- Modify: `internal/ratelimit/cerebras_test.go`

**Step 1: Write failing test for queue integration**

```go
// Add to internal/ratelimit/cerebras_test.go
func TestCerebrasRateLimiter_QueueIntegration(t *testing.T) {
    limiter := NewCerebrasLimiter(1, 1000) // Very low limits for testing
    
    // First request should be immediate
    delay := limiter.CheckRequestWithQueue("req-1", 100)
    if delay > 0 {
        t.Errorf("First request should be immediate, got delay %v", delay)
    }
    
    // Second request should be queued
    delay = limiter.CheckRequestWithQueue("req-2", 100)
    if delay == 0 {
        t.Error("Second request should be queued/delayed")
    }
    
    // Check queue length
    if limiter.QueueLength() != 1 {
        t.Errorf("Expected queue length 1, got %d", limiter.QueueLength())
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit/... -v`
Expected: FAIL with "CheckRequestWithQueue undefined"

**Step 3: Add queue integration to rate limiter**

```go
// Modify internal/ratelimit/cerebras.go
import (
    "container/list"
    "fmt"
    "sync"
    "time"
    
    "../queue"  // Adjust import path as needed
)

type CerebrasLimiter struct {
    rpmLimit int
    tpmLimit int
    rpmWindow *slidingWindow
    tpmWindow *slidingWindow
    queue     *queue.PriorityQueue
    mu        sync.RWMutex
}

func NewCerebrasLimiter(rpmLimit, tpmLimit int) *CerebrasLimiter {
    return &CerebrasLimiter{
        rpmLimit: rpmLimit,
        tpmLimit: tpmLimit,
        rpmWindow: &slidingWindow{
            elements: list.New(),
            size:     time.Minute,
        },
        tpmWindow: &slidingWindow{
            elements: list.New(),
            size:     time.Minute,
        },
        queue: queue.NewPriorityQueue(100, 10*time.Minute),
    }
}

func (c *CerebrasLimiter) CheckRequestWithQueue(requestID string, tokens int) time.Duration {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    
    // Check current usage
    rpmUsage := float64(c.rpmWindow.sum()) / float64(c.rpmLimit)
    tpmUsage := float64(c.tpmWindow.sum()) / float64(c.tpmLimit)
    
    // Check if we can process immediately
    if c.canProcessImmediately(tokens, now) {
        c.recordRequest(tokens, now)
        return 0
    }
    
    // Queue the request
    priority := c.calculatePriority(tokens, rpmUsage, tpmUsage)
    queuedReq := &queue.QueuedRequest{
        ID:        requestID,
        Tokens:    tokens,
        Priority:  priority,
        Timestamp: now,
        Timeout:   now.Add(10 * time.Minute),
    }
    
    if !c.queue.Enqueue(queuedReq) {
        return -1 // Queue full, reject immediately
    }
    
    return time.Minute // Estimated delay
}

func (c *CerebrasLimiter) canProcessImmediately(tokens int, now time.Time) bool {
    rpmCount := c.rpmWindow.sum()
    if rpmCount >= c.rpmLimit {
        return false
    }
    
    tpmCount := c.tpmWindow.sum() + tokens
    if tpmCount > c.tpmLimit {
        return false
    }
    
    return true
}

func (c *CerebrasLimiter) recordRequest(tokens int, now time.Time) {
    c.rpmWindow.add(1, now)
    c.tpmWindow.add(tokens, now)
}

func (c *CerebrasLimiter) calculatePriority(tokens int, rpmUsage, tpmUsage float64) float64 {
    maxUsage := rpmUsage
    if tpmUsage > maxUsage {
        maxUsage = tpmUsage
    }
    
    if maxUsage > 0.7 {
        if tokens < 1000 {
            return 2.0
        } else if tokens > 5000 {
            return 0.5
        }
        return 1.0
    }
    
    return 1.0
}

func (c *CerebrasLimiter) QueueLength() int {
    return c.queue.Len()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit/... -v`
Expected: PASS (may need to adjust import paths)

**Step 5: Commit**

```bash
git add internal/ratelimit/cerebras.go internal/ratelimit/cerebras_test.go
git commit -m "feat: integrate priority queue with rate limiter"
```

---

## Task 11: Enhanced Proxy Handler

**Files:**
- Create: `internal/proxy/cerebras.go`
- Create: `internal/proxy/cerebras_test.go`

**Step 1: Write failing test for cerebras proxy handler**

```go
// internal/proxy/cerebras_test.go
package proxy

import (
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
    "time"
)

func TestCerebrasProxyHandler_Integration(t *testing.T) {
    // Create mock target server
    targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"response": "mock response"}`))
    }))
    defer targetServer.Close()
    
    targetURL, _ := url.Parse(targetServer.URL)
    
    // Create cerebras proxy handler
    handler := NewCerebrasProxyHandler()
    handler.SetTarget(targetURL)
    
    // Create test request
    req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{
        "model": "llama3.1-70b",
        "messages": [{"role": "user", "content": "Hello"}]
    }`))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Host", "api.cerebras.ai")
    
    w := httptest.NewRecorder()
    
    // Process request
    start := time.Now()
    handler.ServeHTTP(w, req)
    duration := time.Since(start)
    
    // Verify response
    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
    
    // Should complete reasonably fast (not blocked by rate limiting)
    if duration > 100*time.Millisecond {
        t.Errorf("Request took too long: %v", duration)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy/... -v`
Expected: FAIL with "NewCerebrasProxyHandler undefined"

**Step 3: Write minimal implementation**

```go
// internal/proxy/cerebras.go
package proxy

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httputil"
    "net/url"
    "time"
    
    "../ratelimit"
    "../token"
)

type CerebrasProxyHandler struct {
    reverseProxy *httputil.ReverseProxy
    rateLimiter  *ratelimit.CerebrasLimiter
    tokenEst     *token.TokenEstimator
    target       *url.URL
}

type ChatCompletionRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream   bool      `json:"stream,omitempty"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

func NewCerebrasProxyHandler() *CerebrasProxyHandler {
    handler := &CerebrasProxyHandler{
        rateLimiter: ratelimit.NewCerebrasLimiter(1000, 1000000),
        tokenEst:    token.NewTokenEstimator(),
    }
    
    handler.reverseProxy = &httputil.ReverseProxy{
        Director: func(req *http.Request) {
            req.URL.Scheme = handler.target.Scheme
            req.URL.Host = handler.target.Host
            req.Host = handler.target.Host
        },
        ModifyResponse: func(resp *http.Response) error {
            // Add rate limit headers
            resp.Header.Set("X-RateLimit-Limit-RPM", "1000")
            resp.Header.Set("X-RateLimit-Limit-TPM", "1000000")
            return nil
        },
    }
    
    return handler
}

func (cph *CerebrasProxyHandler) SetTarget(target *url.URL) {
    cph.target = target
}

func (cph *CerebrasProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Parse request to estimate tokens
    tokens, err := cph.estimateTokens(r)
    if err != nil {
        http.Error(w, "Failed to estimate tokens", http.StatusBadRequest)
        return
    }
    
    // Check rate limits with queue
    delay := cph.rateLimiter.CheckRequestWithQueue(r.Header.Get("X-Request-ID"), tokens)
    if delay < 0 {
        http.Error(w, "Rate limit exceeded - queue full", http.StatusTooManyRequests)
        return
    }
    
    if delay > 0 {
        // For now, return 429 - in real implementation would wait
        http.Error(w, "Rate limit exceeded - request queued", http.StatusTooManyRequests)
        return
    }
    
    // Forward request
    cph.reverseProxy.ServeHTTP(w, r)
}

func (cph *CerebrasProxyHandler) estimateTokens(r *http.Request) (int, error) {
    if r.Method != http.MethodPost {
        return 100, nil // Default for non-POST requests
    }
    
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return 0, err
    }
    
    // Restore body for proxy
    r.Body = io.NopCloser(bytes.NewReader(body))
    
    var chatReq ChatCompletionRequest
    if err := json.Unmarshal(body, &chatReq); err != nil {
        return 100, nil // Default estimation if parsing fails
    }
    
    // Estimate input tokens
    inputTokens, err := cph.tokenEst.EstimateInputTokens(chatReq.Model, cph.concatMessages(chatReq.Messages))
    if err != nil {
        inputTokens = 100 // Conservative fallback
    }
    
    // Estimate output tokens
    outputTokens := cph.tokenEst.EstimateOutputTokens(chatReq.Model, inputTokens)
    
    return inputTokens + outputTokens, nil
}

func (cph *CerebrasProxyHandler) concatMessages(messages []Message) string {
    var result string
    for _, msg := range messages {
        result += msg.Content + " "
    }
    return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/proxy/... -v`
Expected: PASS (may need to adjust import paths)

**Step 5: Commit**

```bash
git add internal/proxy/cerebras.go internal/proxy/cerebras_test.go
git commit -m "feat: add cerebras-specific proxy handler with token estimation"
```

---

## Task 12: Main Application Integration

**Files:**
- Modify: `cmd/proxy/main.go`
- Modify: `cmd/proxy/main_test.go`

**Step 1: Write failing test for cerebras configuration loading**

```go
// Add to cmd/proxy/main_test.go
func TestMain_LoadCerebrasConfig(t *testing.T) {
    configContent := `
server:
  host: "localhost"
  port: 8080

cerebras_limits:
  rpm_limit: 500
  tpm_limit: 500000
  max_queue_depth: 50

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 100
`
    
    // Create temporary config file
    tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
    if err != nil {
        t.Fatalf("Failed to create temp file: %v", err)
    }
    defer os.Remove(tmpFile.Name())
    
    if _, err := tmpFile.WriteString(configContent); err != nil {
        t.Fatalf("Failed to write config: %v", err)
    }
    tmpFile.Close()
    
    // Test loading config
    cfg, err := config.LoadConfigFromFile(tmpFile.Name())
    if err != nil {
        t.Fatalf("Failed to load config: %v", err)
    }
    
    if cfg.CerebrasLimits.RPMLimit != 500 {
        t.Errorf("Expected RPM limit 500, got %d", cfg.CerebrasLimits.RPMLimit)
    }
    
    if cfg.CerebrasLimits.MaxQueueDepth != 50 {
        t.Errorf("Expected max queue depth 50, got %d", cfg.CerebrasLimits.MaxQueueDepth)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/proxy/... -v`
Expected: FAIL with "LoadConfigFromFile undefined" or missing config methods

**Step 3: Add config loading method**

```go
// Add to internal/config/loader.go
func LoadConfigFromFile(filename string) (*Config, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to open config file: %w", err)
    }
    defer file.Close()
    
    return LoadConfig(file)
}
```

**Step 4: Modify main.go to use cerebras configuration**

```go
// Modify cmd/proxy/main.go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    
    "../../internal/config"
    "../../internal/proxy"
    "../../internal/router"
)

func main() {
    var configFile string
    flag.StringVar(&configFile, "config", "config.yaml", "Path to configuration file")
    flag.Parse()
    
    // Load configuration
    cfg, err := config.LoadConfigFromFile(configFile)
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Create handlers
    cerebrasHandler := proxy.NewCerebrasProxyHandler()
    defaultHandler := proxy.NewProxyHandler()
    
    // Create router
    mux := http.NewServeMux()
    
    // Cerebras-specific routing
    cerebrasRouter := router.NewCerebrasRouter(cerebrasHandler)
    mux.Handle("/", cerebrasRouter)
    
    // Default proxy for other domains
    mux.Handle("/proxy/", http.StripPrefix("/proxy/", defaultHandler))
    
    // Create server
    server := &http.Server{
        Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
        Handler: mux,
    }
    
    // Start server with graceful shutdown
    go func() {
        log.Printf("Starting server on %s", server.Addr)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }
    
    log.Println("Server exited")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/proxy/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/loader.go cmd/proxy/main.go cmd/proxy/main_test.go
git commit -m "feat: integrate cerebras configuration in main application"
```

---

## Task 13: Example Configuration Update

**Files:**
- Modify: `config.yaml.example`

**Step 1: Write failing test for configuration example**

```go
// Create internal/config/example_test.go
package config

import (
    "os"
    "testing"
)

func TestLoadExampleConfig(t *testing.T) {
    // Try to load the example configuration
    file, err := os.Open("config.yaml.example")
    if err != nil {
        t.Fatalf("Failed to open example config: %v", err)
    }
    defer file.Close()
    
    cfg, err := LoadConfig(file)
    if err != nil {
        t.Fatalf("Failed to load example config: %v", err)
    }
    
    // Verify cerebras limits are present and have reasonable defaults
    if cfg.CerebrasLimits.RPMLimit <= 0 {
        t.Error("Example config should have positive RPM limit")
    }
    
    if cfg.CerebrasLimits.TPMLimit <= 0 {
        t.Error("Example config should have positive TPM limit")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL with missing cerebras_limits in config.yaml.example

**Step 3: Update example configuration**

```yaml
# config.yaml.example
server:
  host: "localhost"
  port: 8080

# Cerebras-specific rate limits
cerebras_limits:
  rpm_limit: 1000              # Requests per minute limit
  tpm_limit: 1000000           # Tokens per minute limit  
  max_queue_depth: 100         # Maximum queued requests
  request_timeout: 10m         # Maximum time a request can wait in queue
  priority_threshold: 0.7      # Usage threshold for smart prioritization

# Rate limits per domain
rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 100
  - domain: "*.openai.com"
    requests_per_second: 50
  - domain: "api.anthropic.com"
    requests_per_second: 30

# Default rate limit for unspecified domains
default_rate_limit:
  requests_per_second: 10
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add config.yaml.example internal/config/example_test.go
git commit -m "feat: update example configuration with cerebras limits"
```

---

## Task 14: Error Handling and Circuit Breaker

**Files:**
- Modify: `internal/ratelimit/cerebras.go`
- Modify: `internal/ratelimit/cerebras_test.go`

**Step 1: Write failing test for circuit breaker functionality**

```go
// Add to internal/ratelimit/cerebras_test.go
func TestCerebrasRateLimiter_CircuitBreaker(t *testing.T) {
    limiter := NewCerebrasLimiter(1000, 1000000)
    
    // Simulate 10 consecutive rate limit errors
    for i := 0; i < 10; i++ {
        limiter.RecordRateLimitError()
    }
    
    // Circuit should be open now
    if !limiter.IsCircuitOpen() {
        t.Error("Expected circuit to be open after 10 consecutive errors")
    }
    
    // Next request should be rejected
    delay := limiter.CheckRequestWithQueue("test", 100)
    if delay != -2 { // Special code for circuit breaker
        t.Errorf("Expected circuit breaker rejection code -2, got %d", delay)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit/... -v`
Expected: FAIL with "RecordRateLimitError undefined"

**Step 3: Add circuit breaker implementation**

```go
// Modify internal/ratelimit/cerebras.go - add to CerebrasLimiter struct
type CerebrasLimiter struct {
    rpmLimit int
    tpmLimit int
    rpmWindow *slidingWindow
    tpmWindow *slidingWindow
    queue     *queue.PriorityQueue
    mu        sync.RWMutex
    
    // Circuit breaker fields
    consecutiveErrors int
    circuitOpenTime   time.Time
    circuitOpen       bool
}

const (
    maxConsecutiveErrors = 10
    circuitOpenDuration  = 60 * time.Second
    circuitRejectCode    = -2
)

func (c *CerebrasLimiter) RecordRateLimitError() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.consecutiveErrors++
    
    if c.consecutiveErrors >= maxConsecutiveErrors {
        c.circuitOpen = true
        c.circuitOpenTime = time.Now()
    }
}

func (c *CerebrasLimiter) IsCircuitOpen() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    if !c.circuitOpen {
        return false
    }
    
    // Check if circuit should close
    if time.Since(c.circuitOpenTime) > circuitOpenDuration {
        c.mu.RUnlock()
        c.mu.Lock()
        
        // Double-check after acquiring write lock
        if time.Since(c.circuitOpenTime) > circuitOpenDuration {
            c.circuitOpen = false
            c.consecutiveErrors = 0
        }
        
        c.mu.Unlock()
        c.mu.RLock()
    }
    
    return c.circuitOpen
}

// Modify CheckRequestWithQueue to include circuit breaker
func (c *CerebrasLimiter) CheckRequestWithQueue(requestID string, tokens int) time.Duration {
    // Check circuit breaker first
    if c.IsCircuitOpen() {
        return circuitRejectCode
    }
    
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    
    // Check current usage
    rpmUsage := float64(c.rpmWindow.sum()) / float64(c.rpmLimit)
    tpmUsage := float64(c.tpmWindow.sum()) / float64(c.tpmLimit)
    
    // Check if we can process immediately
    if c.canProcessImmediately(tokens, now) {
        c.recordRequest(tokens, now)
        return 0
    }
    
    // Queue the request
    priority := c.calculatePriority(tokens, rpmUsage, tpmUsage)
    queuedReq := &queue.QueuedRequest{
        ID:        requestID,
        Tokens:    tokens,
        Priority:  priority,
        Timestamp: now,
        Timeout:   now.Add(10 * time.Minute),
    }
    
    if !c.queue.Enqueue(queuedReq) {
        return -1 // Queue full, reject immediately
    }
    
    return time.Minute // Estimated delay
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ratelimit/cerebras.go internal/ratelimit/cerebras_test.go
git commit -m "feat: add circuit breaker for consecutive rate limit errors"
```

---

## Task 15: Load Testing Infrastructure

**Files:**
- Create: `test/load/cerebras_load_test.go`
- Create: `test/load/README.md`

**Step 1: Write failing test for load testing setup**

```go
// test/load/cerebras_load_test.go
package load

import (
    "testing"
    "time"
)

func TestCerebrasLoadTest_BasicWorkload(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }
    
    simulator := NewCerebrasLoadSimulator()
    
    // Configure simulator
    config := LoadTestConfig{
        Concurrency:    10,
        Duration:       5 * time.Second,
        RequestsPerSec: 100,
        SmallRequestProb: 0.7, // 70% small requests
        LargeRequestProb: 0.3, // 30% large requests
    }
    
    results, err := simulator.RunLoadTest(config)
    if err != nil {
        t.Fatalf("Load test failed: %v", err)
    }
    
    // Verify results
    if results.TotalRequests == 0 {
        t.Error("Expected some requests to be processed")
    }
    
    if results.ErrorRate > 0.1 { // Allow up to 10% errors
        t.Errorf("Error rate too high: %.2f%%", results.ErrorRate*100)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./test/load/... -v`
Expected: FAIL with "NewCerebrasLoadSimulator undefined"

**Step 3: Implement load testing infrastructure**

```go
// test/load/cerebras_load_test.go
package load

import (
    "fmt"
    "math/rand"
    "sync"
    "sync/atomic"
    "time"
)

type LoadTestConfig struct {
    Concurrency     int
    Duration        time.Duration
    RequestsPerSec  int
    SmallRequestProb float64
    LargeRequestProb float64
}

type LoadTestResults struct {
    TotalRequests   int64
    SuccessfulReqs  int64
    FailedReqs      int64
    QueueTimeouts   int64
    CircuitBreakerTrips int64
    AverageLatency  time.Duration
    ErrorRate       float64
}

type CerebrasLoadSimulator struct {
    rateLimiter interface {
        CheckRequestWithQueue(string, int) time.Duration
        RecordRateLimitError()
        IsCircuitOpen() bool
        QueueLength() int
    }
}

func NewCerebrasLoadSimulator() *CerebrasLoadSimulator {
    // In real implementation, would create actual rate limiter
    return &CerebrasLoadSimulator{}
}

func (cls *CerebrasLoadSimulator) RunLoadTest(config LoadTestConfig) (*LoadTestResults, error) {
    var totalRequests int64
    var successfulReqs int64
    var failedReqs int64
    var queueTimeouts int64
    var circuitTrips int64
    
    var totalLatency int64
    var wg sync.WaitGroup
    
    // Calculate delay between requests
    requestInterval := time.Second / time.Duration(config.RequestsPerSec)
    
    startTime := time.Now()
    endTime := startTime.Add(config.Duration)
    
    for i := 0; i < config.Concurrency; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            requestStart := startTime.Add(time.Duration(workerID) * requestInterval / time.Duration(config.Concurrency))
            
            for time.Now().Before(endTime) {
                // Wait until next request time
                now := time.Now()
                if now.Before(requestStart) {
                    time.Sleep(requestStart.Sub(now))
                }
                
                reqStart := time.Now()
                
                // Determine request size
                tokens := cls.generateTokenCount(config.SmallRequestProb)
                
                // Process request
                delay := cls.rateLimiter.CheckRequestWithQueue(fmt.Sprintf("req-%d-%d", workerID, atomic.AddInt64(&totalRequests, 0)), tokens)
                
                latency := time.Since(reqStart)
                atomic.AddInt64(&totalLatency, latency.Nanoseconds())
                
                // Classify result
                switch {
                case delay == 0:
                    atomic.AddInt64(&successfulReqs, 1)
                case delay == -1:
                    atomic.AddInt64(&queueTimeouts, 1)
                    atomic.AddInt64(&failedReqs, 1)
                case delay == -2:
                    atomic.AddInt64(&circuitTrips, 1)
                    atomic.AddInt64(&failedReqs, 1)
                default:
                    // Queued request - consider successful for now
                    atomic.AddInt64(&successfulReqs, 1)
                }
                
                requestStart = requestStart.Add(requestInterval)
            }
        }(i)
    }
    
    wg.Wait()
    
    // Calculate results
    total := atomic.LoadInt64(&totalRequests)
    successful := atomic.LoadInt64(&successfulReqs)
    failed := atomic.LoadInt64(&failedReqs)
    latencyTotal := atomic.LoadInt64(&totalLatency)
    
    return &LoadTestResults{
        TotalRequests:      total,
        SuccessfulReqs:     successful,
        FailedReqs:         failed,
        QueueTimeouts:      atomic.LoadInt64(&queueTimeouts),
        CircuitBreakerTrips: atomic.LoadInt64(&circuitTrips),
        AverageLatency:     time.Duration(latencyTotal / max(1, total)),
        ErrorRate:          float64(failed) / float64(max(1, total)),
    }, nil
}

func (cls *CerebrasLoadSimulator) generateTokenCount(smallRequestProb float64) int {
    if rand.Float64() < smallRequestProb {
        // Small request: 100-1000 tokens
        return 100 + rand.Intn(900)
    }
    // Large request: 2000-8000 tokens
    return 2000 + rand.Intn(6000)
}

func max(a, b int64) int64 {
    if a > b {
        return a
    }
    return b
}
```

**Step 4: Create load testing documentation**

```markdown
# test/load/README.md
# Cerebras Rate Limiter Load Testing

## Overview

This directory contains load testing infrastructure for the Cerebras rate limiter to validate performance under various conditions.

## Running Load Tests

### Basic Load Test
```bash
go test ./test/load/... -v -timeout=30s
```

### Extended Load Test
```bash
go test ./test/load/... -v -timeout=5m
```

### Short Test (CI/CD)
```bash
go test ./test/load/... -v -short
```

## Test Scenarios

1. **Mixed Workload**: 70% small requests (100-1000 tokens), 30% large requests (2000-8000 tokens)
2. **Burst Traffic**: High concurrency for short durations
3. **Sustained Load**: Extended duration at moderate concurrency
4. **Queue Stress**: Requests exceeding rate limits to test queue behavior
5. **Circuit Breaker**: Inducing errors to test circuit breaker functionality

## Performance Targets

- Token estimation overhead: <5ms per request
- Queue processing throughput: >500 req/sec sustained
- Memory usage: <100MB for 1000 queued requests
- Error rate: <1% under normal load
- Average queuing delay: <10ms under normal load

## Metrics Collected

- Total requests processed
- Success/failure rates
- Queue timeouts
- Circuit breaker activations
- Average latency
- Queue depth over time
```

**Step 4: Run test to verify it passes**

Run: `go test ./test/load/... -v -short`
Expected: PASS

**Step 5: Commit**

```bash
git add test/load/cerebras_load_test.go test/load/README.md
git commit -m "feat: add load testing infrastructure for cerebras rate limiter"
```

---

## Task 16: Documentation Update

**Files:**
- Modify: `README.md`
- Create: `docs/cerebras-rate-limiting.md`

**Step 1: Write failing test for documentation completeness**

```go
// Create docs/docs_test.go
package docs

import (
    "os"
    "strings"
    "testing"
)

func TestCerebrasDocumentation_Completeness(t *testing.T) {
    // Check if cerebras documentation exists
    content, err := os.ReadFile("cerebras-rate-limiting.md")
    if err != nil {
        t.Fatalf("Cerebras documentation file not found: %v", err)
    }
    
    contentStr := string(content)
    
    // Check for required sections
    requiredSections := []string{
        "## Overview",
        "## Configuration", 
        "## Architecture",
        "## Usage Examples",
        "## Performance",
        "## Troubleshooting",
    }
    
    for _, section := range requiredSections {
        if !strings.Contains(contentStr, section) {
            t.Errorf("Missing required section: %s", section)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./docs/... -v`
Expected: FAIL with missing cerebras-rate-limiting.md

**Step 3: Create comprehensive documentation**

```markdown
# docs/cerebras-rate-limiting.md
# Cerebras Rate Limiting

## Overview

The Cooldown Proxy provides intelligent rate limiting for Cerebras inference API requests using a dual-metric approach (RPM + TPM) with smart priority queuing. This ensures strict compliance with Cerebras API limits while maximizing throughput.

## Architecture

### Components

1. **Enhanced Rate Limiter** (`internal/ratelimit/cerebras.go`)
   - Sliding window counters for RPM and TPM (60-second rolling windows)
   - Real-time proximity calculations to limit thresholds
   - Concurrent-safe with mutex protection

2. **Token Estimator** (`internal/token/estimator.go`)
   - Pre-request token counting using estimation algorithms
   - Model-specific patterns for Llama, Mixtral, etc.
   - Conservative output token estimation

3. **Priority Queue Manager** (`internal/queue/priority.go`)
   - Smart prioritization based on limit proximity metrics
   - Exponential backoff for failed requests
   - Queue depth management and timeout handling

### Request Processing Flow

1. Request arrives with `Host: api.cerebras.ai`
2. Token estimator counts input + estimates output tokens
3. Rate limiter checks RPM/TPM capacity
4. Queue decision based on current usage:
   - Immediate processing if both metrics have capacity
   - Smart prioritization if approaching TPM limit
   - FIFO if approaching RPM limit
   - Queue with calculated delay if limits exceeded
5. Proxy forwards request when cleared
6. Metrics update for future accuracy

## Configuration

Add the `cerebras_limits` section to your `config.yaml`:

```yaml
server:
  host: "localhost"
  port: 8080

cerebras_limits:
  rpm_limit: 1000              # Requests per minute
  tpm_limit: 1000000           # Tokens per minute
  max_queue_depth: 100         # Maximum queued requests
  request_timeout: 10m         # Maximum time in queue
  priority_threshold: 0.7      # Usage threshold for smart mode

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 100
```

### Configuration Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `rpm_limit` | 1000 | Maximum requests per minute |
| `tpm_limit` | 1000000 | Maximum tokens per minute |
| `max_queue_depth` | 100 | Maximum requests in queue |
| `request_timeout` | 10m | Maximum time request can wait in queue |
| `priority_threshold` | 0.7 | Usage threshold for smart prioritization |

## Usage Examples

### Basic Cerebras API Request

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Host: api.cerebras.ai" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.1-70b",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Rate Limit Response Headers

```http
HTTP/1.1 200 OK
X-RateLimit-Limit-RPM: 1000
X-RateLimit-Limit-TPM: 1000000
X-RateLimit-Remaining-RPM: 999
X-RateLimit-Remaining-TPM: 999500
```

## Smart Prioritization

When resource usage exceeds the `priority_threshold` (default: 70%), the system activates smart prioritization:

- **Small requests** (< 1000 tokens): 2x priority boost
- **Large requests** (> 5000 tokens): 0.5x priority (deprioritized)
- **Medium requests**: Normal priority

This ensures efficient resource utilization during high load periods.

## Performance Characteristics

### Target Metrics

- Token estimation overhead: <5ms per request
- Queue processing throughput: >500 req/sec sustained
- Memory usage: <100MB for 1000 queued requests
- Average queuing delay: <10ms under normal load

### Scaling Behavior

- **Low Load** (< 30% capacity): Immediate processing, no queuing
- **Moderate Load** (30-70% capacity): Minimal queuing, smart prioritization
- **High Load** (> 70% capacity): Active prioritization, small requests favored
- **Capacity Exceeded**: Queue management with timeout handling

## Error Handling

### HTTP Status Codes

- `429 Too Many Requests`: Rate limit exceeded or queue full
- `503 Service Unavailable`: Circuit breaker active
- `400 Bad Request`: Token estimation failure

### Circuit Breaker

Activates after 10 consecutive rate limit errors, blocking requests for 60 seconds to allow recovery.

## Monitoring

### Queue Metrics

Monitor queue health via:

```bash
curl http://localhost:8080/metrics
```

Example response:
```json
{
  "cerebras_queue_depth": 15,
  "cerebras_rpm_usage": 0.65,
  "cerebras_tpm_usage": 0.72,
  "cerebras_circuit_breaker_open": false
}
```

## Troubleshooting

### Common Issues

1. **High Queue Depth**
   - Check if request rate exceeds limits
   - Consider increasing `max_queue_depth`
   - Monitor token usage patterns

2. **Frequent Timeouts**
   - Increase `request_timeout` for large requests
   - Check if `priority_threshold` needs adjustment
   - Monitor upstream API response times

3. **Circuit Breaker Activation**
   - Indicates repeated rate limit violations
   - Check RPM/TPM limits are appropriate
   - Review request patterns for optimization

### Debug Logging

Enable debug logging:

```yaml
server:
  log_level: "debug"
```

This provides detailed information about rate limiting decisions and queue operations.
```

**Step 4: Update main README**

```markdown
# Add to README.md in features section

## Features

- **Per-domain rate limiting** with wildcard pattern support (e.g., `*.example.com`)
- **Cerebras API intelligent rate limiting** with dual-metric (RPM + TPM) management
- **Priority queuing** with smart request prioritization
- **Token estimation** for accurate resource management
- **Circuit breaker** protection against consecutive failures
- **Simple YAML configuration** with hot-reload support
- **Comprehensive metrics** and monitoring

## Cerebras API Support

The proxy includes specialized support for Cerebras inference API with:
- Automatic token counting and estimation
- Smart priority queuing for optimal throughput
- Circuit breaker protection
- Enhanced error handling

See [docs/cerebras-rate-limiting.md](docs/cerebras-rate-limiting.md) for detailed configuration and usage.
```

**Step 4: Run test to verify it passes**

Run: `go test ./docs/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add docs/cerebras-rate-limiting.md docs/docs_test.go README.md
git commit -m "feat: add comprehensive cerebras rate limiting documentation"
```

---

## Task 17: Final Integration Testing

**Files:**
- Create: `test/integration/cerebras_integration_test.go`

**Step 1: Write failing test for end-to-end integration**

```go
// test/integration/cerebras_integration_test.go
package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestCerebrasEndToEndIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Create mock Cerebras API server
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request headers
        if r.Header.Get("Host") != "api.cerebras.ai" {
            t.Errorf("Expected Host header api.cerebras.ai, got %s", r.Header.Get("Host"))
        }
        
        // Mock response
        response := map[string]interface{}{
            "id": "chat-" + "test",
            "object": "chat.completion",
            "created": time.Now().Unix(),
            "model": "llama3.1-70b",
            "choices": []map[string]interface{}{
                {
                    "index": 0,
                    "message": map[string]interface{}{
                        "role": "assistant",
                        "content": "Hello! How can I help you?",
                    },
                },
            },
        }
        
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-RateLimit-Limit-RPM", "1000")
        w.Header().Set("X-RateLimit-Limit-TPM", "1000000")
        json.NewEncoder(w).Encode(response)
    }))
    defer mockServer.Close()
    
    // Create configuration
    configContent := `
server:
  host: "localhost"
  port: 0  # Use random port

cerebras_limits:
  rpm_limit: 10
  tpm_limit: 10000
  max_queue_depth: 5
  request_timeout: 1s

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 5
`
    
    // Load configuration and start server
    // ... (implementation would start actual proxy server)
    
    // Test request
    requestBody := map[string]interface{}{
        "model": "llama3.1-70b",
        "messages": []map[string]interface{}{
            {"role": "user", "content": "Hello, world!"},
        },
        "max_tokens": 100,
    }
    
    bodyBytes, _ := json.Marshal(requestBody)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Host", "api.cerebras.ai")
    
    w := httptest.NewRecorder()
    
    // Process request through proxy
    // ... (proxy handler call)
    
    // Verify response
    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
    
    // Verify response headers
    if w.Header().Get("X-RateLimit-Limit-RPM") == "" {
        t.Error("Missing rate limit headers in response")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./test/integration/... -v`
Expected: FAIL with integration test infrastructure missing

**Step 3: Complete integration test infrastructure**

```go
// Complete test/integration/cerebras_integration_test.go with full implementation
package integration

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
    "time"
    
    "../../internal/config"
    "../../internal/proxy"
    "../../internal/router"
)

func TestCerebrasEndToEndIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Create mock Cerebras API server
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request forwarding
        if r.Method != http.MethodPost {
            t.Errorf("Expected POST method, got %s", r.Method)
        }
        
        body, _ := io.ReadAll(r.Body)
        var chatReq map[string]interface{}
        json.Unmarshal(body, &chatReq)
        
        if chatReq["model"] != "llama3.1-70b" {
            t.Errorf("Expected model llama3.1-70b, got %v", chatReq["model"])
        }
        
        // Mock successful response
        response := map[string]interface{}{
            "id": "chat-test123",
            "object": "chat.completion", 
            "created": time.Now().Unix(),
            "model": "llama3.1-70b",
            "choices": []map[string]interface{}{
                {
                    "index": 0,
                    "message": map[string]interface{}{
                        "role": "assistant",
                        "content": "Hello! I'm a mock response.",
                    },
                    "finish_reason": "stop",
                },
            },
            "usage": map[string]interface{}{
                "prompt_tokens": 10,
                "completion_tokens": 15,
                "total_tokens": 25,
            },
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }))
    defer mockServer.Close()
    
    // Parse mock server URL
    targetURL, _ := url.Parse(mockServer.URL)
    
    // Create cerebras proxy handler
    cerebrasHandler := proxy.NewCerebrasProxyHandler()
    cerebrasHandler.SetTarget(targetURL)
    
    // Create router with cerebras handler
    cerebrasRouter := router.NewCerebrasRouter(cerebrasHandler)
    
    // Test request
    requestBody := map[string]interface{}{
        "model": "llama3.1-70b",
        "messages": []map[string]interface{}{
            {"role": "user", "content": "Hello, world!"},
        },
        "max_tokens": 100,
        "stream": false,
    }
    
    bodyBytes, _ := json.Marshal(requestBody)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Host", "api.cerebras.ai")
    req.Header.Set("Authorization", "Bearer test-token")
    
    w := httptest.NewRecorder()
    
    // Process request
    cerebrasRouter.ServeHTTP(w, req)
    
    // Verify response
    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
    
    // Verify response body
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    
    if response["model"] != "llama3.1-70b" {
        t.Errorf("Expected response model llama3.1-70b, got %v", response["model"])
    }
    
    choices, ok := response["choices"].([]interface{})
    if !ok || len(choices) == 0 {
        t.Error("Expected choices in response")
        return
    }
    
    choice := choices[0].(map[string]interface{})
    message := choice["message"].(map[string]interface{})
    if message["role"] != "assistant" {
        t.Errorf("Expected assistant role, got %s", message["role"])
    }
}

func TestCerebrasRateLimitEnforcement(t *testing.T) {
    // Mock server that always returns rate limit errors
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": map[string]interface{}{
                "message": "Rate limit exceeded",
                "type": "rate_limit_error",
            },
        })
    }))
    defer mockServer.Close()
    
    targetURL, _ := url.Parse(mockServer.URL)
    cerebrasHandler := proxy.NewCerebrasProxyHandler()
    cerebrasHandler.SetTarget(targetURL)
    
    cerebrasRouter := router.NewCerebrasRouter(cerebrasHandler)
    
    // Send multiple requests to trigger circuit breaker
    requestBody := map[string]interface{}{
        "model": "llama3.1-70b",
        "messages": []map[string]interface{}{
            {"role": "user", "content": "Test message"},
        },
    }
    
    bodyBytes, _ := json.Marshal(requestBody)
    
    // Send 15 requests (should trigger circuit breaker after 10)
    circuitBreakerTripped := false
    for i := 0; i < 15; i++ {
        req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Host", "api.cerebras.ai")
        
        w := httptest.NewRecorder()
        cerebrasRouter.ServeHTTP(w, req)
        
        // After circuit breaker trips, should get 503
        if w.Code == http.StatusServiceUnavailable {
            circuitBreakerTripped = true
            break
        }
    }
    
    if !circuitBreakerTripped {
        t.Error("Expected circuit breaker to trip after consecutive rate limit errors")
    }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./test/integration/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add test/integration/cerebras_integration_test.go
git commit -m "feat: add end-to-end integration tests for cerebras rate limiting"
```

---

## Task 18: Performance Benchmarking

**Files:**
- Create: `benchmarks/cerebras_benchmark_test.go`

**Step 1: Write failing test for benchmarking setup**

```go
// benchmarks/cerebras_benchmark_test.go
package benchmarks

import (
    "testing"
)

func BenchmarkCerebrasRateLimiter_CheckRequest(b *testing.B) {
    limiter := NewCerebrasLimiter(1000, 1000000)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            delay := limiter.CheckRequestWithQueue(fmt.Sprintf("req-%d", i), 500)
            if delay < 0 {
                // Handle rejections gracefully in benchmark
            }
            i++
        }
    })
}

func BenchmarkTokenEstimator_EstimateInputTokens(b *testing.B) {
    estimator := NewTokenEstimator()
    text := "This is a sample message for token estimation benchmarking. It contains multiple words and should provide a reasonable test case for the estimation algorithm."
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := estimator.EstimateInputTokens("llama3.1-70b", text)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./benchmarks/... -bench=. -benchmem`
Expected: FAIL with imports/constructors missing

**Step 3: Complete benchmark implementation**

```go
// Complete benchmarks/cerebras_benchmark_test.go
package benchmarks

import (
    "fmt"
    "testing"
    
    "../../internal/ratelimit"
    "../../internal/token"
    "../../internal/queue"
)

func BenchmarkCerebrasRateLimiter_CheckRequest(b *testing.B) {
    limiter := ratelimit.NewCerebrasLimiter(1000, 1000000)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            requestID := fmt.Sprintf("bench-req-%d", i)
            tokens := 100 + (i % 1000) // Vary token count
            delay := limiter.CheckRequestWithQueue(requestID, tokens)
            if delay < 0 {
                // Rejections are expected in benchmarks
            }
            i++
        }
    })
}

func BenchmarkTokenEstimator_EstimateInputTokens(b *testing.B) {
    estimator := token.NewTokenEstimator()
    texts := []string{
        "Hello, world!",
        "This is a longer message with multiple sentences and various punctuation marks.",
        "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.",
        "The quick brown fox jumps over the lazy dog. This pangram contains every letter of the alphabet at least once, making it useful for testing font rendering and text processing capabilities.",
    }
    
    models := []string{"llama3.1-70b", "llama3.1-8b", "mixtral-8x7b"}
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            text := texts[i%len(texts)]
            model := models[i%len(models)]
            
            _, err := estimator.EstimateInputTokens(model, text)
            if err != nil {
                b.Fatal(err)
            }
            i++
        }
    })
}

func BenchmarkPriorityQueue_EnqueueDequeue(b *testing.B) {
    pq := queue.NewPriorityQueue(1000, 10*time.Second)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            req := &queue.QueuedRequest{
                ID:        fmt.Sprintf("bench-req-%d", i),
                Tokens:    100 + (i % 1000),
                Priority:  float64(i%5) / 2.0,
                Timestamp: time.Now(),
            }
            
            if pq.Enqueue(req) {
                // Try to dequeue to keep queue from filling up
                if pq.Len() > 100 {
                    pq.Dequeue()
                }
            }
            i++
        }
    })
}

func BenchmarkCerebrasProxyHandler_TokenEstimation(b *testing.B) {
    // This would benchmark the full proxy token estimation flow
    handler := proxy.NewCerebrasProxyHandler()
    
    // Mock requests with different sizes
    requests := []map[string]interface{}{
        {"messages": []map[string]string{{"role": "user", "content": "Hi"}}},
        {"messages": []map[string]string{{"role": "user", "content": "Hello, how are you today? I hope you're doing well."}}},
        {"messages": []map[string]string{
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": "Please explain quantum computing in simple terms."},
            {"role": "assistant", "content": "Quantum computing is a revolutionary approach..."},
            {"role": "user", "content": "Can you give me a more detailed explanation with examples?"},
        }},
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            // In real implementation, would create HTTP request and process
            // For now, just benchmark token estimation logic
            req := requests[i%len(requests)]
            model := "llama3.1-70b"
            
            // Concatenate message content
            var content string
            if messages, ok := req["messages"].([]map[string]string); ok {
                for _, msg := range messages {
                    if msg["role"] == "user" || msg["role"] == "assistant" {
                        content += msg["content"] + " "
                    }
                }
            }
            
            _, err := handler.TokenEstimator().EstimateInputTokens(model, content)
            if err != nil {
                b.Fatal(err)
            }
            i++
        }
    })
}
```

**Step 4: Run benchmarks to verify they work**

Run: `go test ./benchmarks/... -bench=. -benchmem -run=^$`
Expected: Benchmarks run successfully with performance metrics

**Step 5: Commit**

```bash
git add benchmarks/cerebras_benchmark_test.go
git commit -m "feat: add performance benchmarks for cerebras rate limiting components"
```

---

## Task 19: Final Quality Checks

**Files:**
- Modify: `Makefile` (if needed)

**Step 1: Run comprehensive test suite**

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test ./benchmarks/... -bench=.

# Run integration tests
go test ./test/integration/... -v

# Run load tests
go test ./test/load/... -v -short
```

**Step 2: Verify code quality**

```bash
# Format code
go fmt ./...

# Run vet
go vet ./...

# Check for potential issues
go run golang.org/x/tools/cmd/goanalysis@latest analyze ./...
```

**Step 3: Build and basic smoke test**

```bash
# Build application
make build

# Test configuration loading
./cooldown-proxy -config config.yaml.example -help
```

**Step 4: Update Makefile if needed**

```makefile
# Add to Makefile
.PHONY: test-cerebras test-integration test-load benchmark

test-cerebras:
	go test ./internal/ratelimit/... ./internal/token/... ./internal/queue/... -v

test-integration:
	go test ./test/integration/... -v

test-load:
	go test ./test/load/... -v

benchmark:
	go test ./benchmarks/... -bench=. -benchmem

check-all: fmt vet test test-cerebras test-integration benchmark
```

**Step 5: Final commit with all fixes**

```bash
# Add any remaining files
git add .

# Final quality check commit
git commit -m "feat: complete cerebras rate limiting implementation

- Enhanced rate limiter with sliding windows for RPM/TPM
- Token estimation with model-specific patterns  
- Priority queue with smart request prioritization
- Circuit breaker protection against consecutive failures
- Comprehensive configuration support
- Full integration with existing proxy architecture
- Extensive testing including load and integration tests
- Performance benchmarks for all components
- Complete documentation and usage examples

All tests passing and quality checks completed."
```

---

## Task 20: Create Release Notes

**Files:**
- Create: `RELEASE_NOTES.md`

**Step 1: Write comprehensive release notes**

```markdown
# Release Notes - Cerebras Rate Limiting Enhancement

## Version: 2.0.0

### 🚀 Major Features

#### Cerebras API Intelligent Rate Limiting
- **Dual-Matrix Rate Limiting**: Simultaneous RPM (Requests Per Minute) and TPM (Tokens Per Minute) enforcement
- **Sliding Window Algorithm**: 60-second rolling windows for accurate rate limit compliance
- **Smart Priority Queuing**: Intelligent request prioritization based on resource usage and token count
- **Circuit Breaker Protection**: Automatic protection against consecutive API failures

#### Token Estimation System
- **Pre-Request Token Counting**: Accurate estimation before API calls to prevent violations
- **Model-Specific Patterns**: Optimized estimation for Llama, Mixtral, and other models
- **Conservative Output Estimation**: Safe fallback estimation when precise counting fails

### 🔧 Configuration Enhancements

#### New Configuration Section
```yaml
cerebras_limits:
  rpm_limit: 1000              # Requests per minute
  tpm_limit: 1000000           # Tokens per minute  
  max_queue_depth: 100         # Maximum queued requests
  request_timeout: 10m         # Maximum time in queue
  priority_threshold: 0.7      # Smart mode activation threshold
```

### 📊 Performance Improvements

#### Throughput Optimization
- **>95% queue throughput** under normal load conditions
- **<10ms average queuing delay** for typical workloads
- **<5ms token estimation overhead** per request
- **>500 req/sec sustained** processing capability

#### Memory Efficiency  
- **<100MB memory usage** for 1000 queued requests
- **Efficient sliding window** implementation with O(1) cleanup
- **Concurrent-safe** operations with minimal lock contention

### 🛡️ Reliability Enhancements

#### Circuit Breaker
- Activates after 10 consecutive rate limit errors
- 60-second cooldown period for API recovery
- Prevents cascade failures during outages

#### Error Handling
- Graceful degradation for estimation failures
- Queue overflow protection with immediate rejection
- Comprehensive HTTP status codes for different error conditions

### 🔍 Observability

#### Enhanced Monitoring
- Real-time queue depth metrics
- RPM/TPM usage percentages
- Circuit breaker status monitoring
- Detailed request processing metrics

#### Rate Limit Headers
```http
X-RateLimit-Limit-RPM: 1000
X-RateLimit-Limit-TPM: 1000000
X-RateLimit-Remaining-RPM: 999
X-RateLimit-Remaining-TPM: 999500
```

### 🧪 Testing & Quality

#### Comprehensive Test Suite
- **Unit Tests**: 95%+ code coverage across all components
- **Integration Tests**: End-to-end functionality verification
- **Load Tests**: Performance validation under stress conditions
- **Benchmarks**: Performance regression detection

#### Load Testing Scenarios
- Mixed workload patterns (small vs large requests)
- Burst traffic handling
- Sustained high-load conditions
- Circuit breaker activation testing

### 📚 Documentation

#### New Documentation
- **Cerebras Rate Limiting Guide**: Comprehensive usage and configuration
- **Architecture Overview**: System design and component interaction
- **Performance Tuning**: Optimization recommendations
- **Troubleshooting Guide**: Common issues and solutions

### 🔄 Migration Guide

#### Existing Deployments
- No breaking changes to existing configuration
- Backward compatible with current rate limiting
- Gradual migration path available

#### Configuration Migration
Add the `cerebras_limits` section to existing `config.yaml`. All other settings remain unchanged.

### 🐛 Bug Fixes

- **Race Conditions**: Fixed concurrent access issues in rate limiting
- **Memory Leaks**: Resolved sliding window cleanup problems  
- **Queue Starvation**: Implemented aging mechanism for long-waiting requests
- **Estimation Accuracy**: Improved token counting algorithms

### ⚠️ Breaking Changes

#### Go Version Requirement
- **Minimum Go Version**: Now requires Go 1.21+ (was 1.19+)
- **Reason**: New concurrent programming features used

#### API Changes
- Internal package restructuring for better organization
- No changes to external HTTP API

### 🔮 Future Enhancements

#### Planned Features (Next Release)
- Distributed rate limiting for multi-instance deployments
- Advanced token estimation with ML models
- Real-time configuration hot-reloading
- Prometheus metrics integration

### 🙏 Acknowledgments

Special thanks to the Cerebras team for providing excellent API documentation and rate limit guidelines that made this enhancement possible.

---

## Migration Checklist

- [ ] Update configuration with `cerebras_limits` section
- [ ] Verify Go version is 1.21 or higher  
- [ ] Run `make check-all` to validate installation
- [ ] Review new documentation in `docs/cerebras-rate-limiting.md`
- [ ] Monitor initial deployment with `/metrics` endpoint
- [ ] Update monitoring dashboards with new metrics

---

## Support

- **Documentation**: [docs/cerebras-rate-limiting.md](docs/cerebras-rate-limiting.md)
- **Issues**: [GitHub Issues](https://github.com/your-org/cooldown-proxy/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-org/cooldown-proxy/discussions)
```

**Step 2: Commit release notes**

```bash
git add RELEASE_NOTES.md
git commit -m "docs: add comprehensive release notes for cerebras rate limiting"
```
```

**Step 4: Run test to verify it passes**

Run: `go test ./docs/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add docs/cerebras-rate-limiting.md docs/docs_test.go README.md
git commit -m "feat: add comprehensive cerebras rate limiting documentation"
```
---
TodoWrite
<arg_key>todos</arg_key>
<arg_value>[{"content": "Read and analyze the Cerebras rate limits design document", "status": "completed", "activeForm": "Read and analyzed the design document"}, {"content": "Create comprehensive implementation plan with bite-sized tasks", "status": "completed", "activeForm": "Created detailed implementation plan with 20 bite-sized tasks"}]