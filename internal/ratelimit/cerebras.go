package ratelimit

import (
	"container/list"
	"sync"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/queue"
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
	rpmLimit  int
	tpmLimit  int
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
