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

func TestPriorityQueue_SmartPriorityCalculation(t *testing.T) {
	pq := NewPriorityQueue(10, 5*time.Minute)

	// Simulate high TPM usage (70% threshold)
	highTPMUsage := 0.8

	// Small request should get priority boost
	smallReq := &QueuedRequest{
		ID:        "small",
		Tokens:    500, // < 1000 tokens
		Timestamp: time.Now(),
	}

	priority := pq.CalculatePriority(smallReq.Tokens, 0.5, highTPMUsage)
	if priority <= 1.0 {
		t.Errorf("Small request should get priority boost, got priority %f", priority)
	}

	// Large request should get priority penalty
	largeReq := &QueuedRequest{
		ID:        "large",
		Tokens:    6000, // > 5000 tokens
		Timestamp: time.Now(),
	}

	priority = pq.CalculatePriority(largeReq.Tokens, 0.5, highTPMUsage)
	if priority >= 1.0 {
		t.Errorf("Large request should get priority penalty, got priority %f", priority)
	}
}
