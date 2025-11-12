package benchmarks

import (
	"container/list"
	"fmt"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/queue"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
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

			tokens, err := estimator.EstimateInputTokens(model, text)
			if err != nil {
				b.Fatal(err)
			}
			if tokens <= 0 {
				b.Fatal("Estimated tokens should be positive")
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
	// Create token estimator directly
	estimator := token.NewTokenEstimator()

	// Mock content with different sizes
	contents := []string{
		"Hi",
		"Hello, how are you today? I hope you're doing well.",
		"You are a helpful assistant. Please explain quantum computing in simple terms. Quantum computing is a revolutionary approach that uses quantum mechanics to process information in ways that classical computers cannot.",
	}

	models := []string{"llama3.1-70b", "llama3.1-8b", "mixtral-8x7b"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			content := contents[i%len(contents)]
			model := models[i%len(models)]

			tokens, err := estimator.EstimateInputTokens(model, content)
			if err != nil {
				b.Fatal(err)
			}
			if tokens <= 0 {
				b.Fatal("Estimated tokens should be positive")
			}
			i++
		}
	})
}

func BenchmarkSlidingWindow_Operations(b *testing.B) {
	// Benchmark the sliding window operations directly
	sw := &slidingWindow{
		elements: list.New(),
		size:     time.Minute,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			now := time.Now()
			sw.add(i%100, now)
			total := sw.sum()
			if total < 0 {
				b.Fatal("Sum should be non-negative")
			}
			i++
		}
	})
}

// Helper structs and types for benchmarking

type slidingWindow struct {
	elements *list.List
	size     time.Duration
}

type windowElement struct {
	timestamp time.Time
	value     int
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
