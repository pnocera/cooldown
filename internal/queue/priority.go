package queue

import (
	"container/heap"
	"sync"
	"time"
)

type QueuedRequest struct {
	ID        string
	Tokens    int
	Priority  float64
	Timestamp time.Time
	Timeout   time.Time
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
