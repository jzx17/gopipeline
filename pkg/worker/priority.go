// Package worker provides priority queue support
package worker

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// PriorityTask represents a task with priority
type PriorityTask struct {
	task       types.Task
	priority   int       // priority level, higher value means higher priority
	SubmitTime time.Time // submit time for FIFO ordering within same priority
	AgeBoost   int32     // age boost value for starvation prevention
}

// NewPriorityTask creates a priority task wrapper
func NewPriorityTask(task types.Task, priority int) *PriorityTask {
	return NewPriorityTaskWithClock(task, priority, types.NewRealClock())
}

// NewPriorityTaskWithClock creates a priority task wrapper with custom clock
func NewPriorityTaskWithClock(task types.Task, priority int, clock types.Clock) *PriorityTask {
	if clock == nil {
		clock = types.NewRealClock()
	}
	return &PriorityTask{
		task:       task,
		priority:   priority,
		SubmitTime: clock.Now(),
		AgeBoost:   0,
	}
}

// NewPriorityTaskFromTask creates priority task from regular task (alias)
func NewPriorityTaskFromTask(task types.Task, priority int) *PriorityTask {
	return NewPriorityTask(task, priority)
}

// Priority returns the base priority
func (pt *PriorityTask) Priority() int {
	return pt.priority
}

// GetEffectivePriority gets effective priority (including age boost)
func (pt *PriorityTask) GetEffectivePriority() int {
	return pt.priority + int(atomic.LoadInt32(&pt.AgeBoost))
}

// BoostPriority boosts priority (for starvation prevention)
func (pt *PriorityTask) BoostPriority(boost int) {
	atomic.AddInt32(&pt.AgeBoost, int32(boost))
}

// ID returns task ID
func (pt *PriorityTask) ID() string {
	return pt.task.ID()
}

// Execute executes the task
func (pt *PriorityTask) Execute(ctx context.Context) error {
	return pt.task.Execute(ctx)
}

// PriorityStats contains priority queue statistics
type PriorityStats struct {
	// queue length by priority level (grouped by priority)
	QueueLengthByPriority map[int]int

	// task wait time statistics
	AverageWaitTime   time.Duration
	MaxWaitTime       time.Duration
	TotalWaitingTasks int64

	// starvation detection metrics
	StarvationDetected int64 // number of tasks detected as starved
	AgePromotions      int64 // total number of age promotions

	// priority distribution
	TasksByPriority     map[int]int64 // total tasks by priority
	CompletedByPriority map[int]int64 // completed tasks by priority
}

// priorityTaskHeap is a priority task heap (internal use)
type priorityTaskHeap []*PriorityTask

// Len implements heap.Interface
func (h priorityTaskHeap) Len() int { return len(h) }

// Less implements heap.Interface - higher priority first, FIFO for same priority
func (h priorityTaskHeap) Less(i, j int) bool {
	iPriority := h[i].GetEffectivePriority()
	jPriority := h[j].GetEffectivePriority()

	// higher priority first
	if iPriority != jPriority {
		return iPriority > jPriority
	}

	// same priority, FIFO by submit time
	return h[i].SubmitTime.Before(h[j].SubmitTime)
}

// Swap implements heap.Interface
func (h priorityTaskHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push implements heap.Interface
func (h *priorityTaskHeap) Push(x interface{}) {
	*h = append(*h, x.(*PriorityTask))
}

// Pop implements heap.Interface
func (h *priorityTaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// PriorityQueue is a priority queue implementation
type PriorityQueue struct {
	heap priorityTaskHeap
	mu   sync.RWMutex

	// statistics
	stats *priorityQueueStats

	// starvation prevention configuration
	starvationConfig *StarvationConfig

	// time operations
	clock types.Clock

	// stop signal
	stopCh   chan struct{}
	stopOnce sync.Once
}

// priorityQueueStats contains internal statistics
type priorityQueueStats struct {
	totalEnqueued    int64
	totalDequeued    int64
	currentSize      int64
	agePromotions    int64
	starvationEvents int64
	totalWaitTime    int64 // nanoseconds
	maxWaitTime      int64 // nanoseconds
}

// StarvationConfig defines starvation prevention configuration
type StarvationConfig struct {
	// MaxWaitTime is the maximum wait time before tasks get priority boost
	MaxWaitTime time.Duration

	// AgePromotionInterval is the interval for age promotion checks
	AgePromotionInterval time.Duration

	// PriorityBoostAmount is the priority boost amount per promotion
	PriorityBoostAmount int

	// MaxPriorityBoost is the maximum priority boost value
	MaxPriorityBoost int

	// Enable controls whether starvation prevention is enabled
	Enable bool
}

// DefaultStarvationConfig returns default starvation prevention configuration
func DefaultStarvationConfig() *StarvationConfig {
	return &StarvationConfig{
		MaxWaitTime:          30 * time.Second,
		AgePromotionInterval: 5 * time.Second,
		PriorityBoostAmount:  1,
		MaxPriorityBoost:     10,
		Enable:               true,
	}
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(config *StarvationConfig) *PriorityQueue {
	return NewPriorityQueueWithClock(config, types.NewRealClock())
}

func NewPriorityQueueWithClock(config *StarvationConfig, clock types.Clock) *PriorityQueue {
	if config == nil {
		config = DefaultStarvationConfig()
	}
	if clock == nil {
		clock = types.NewRealClock()
	}

	pq := &PriorityQueue{
		heap:             make(priorityTaskHeap, 0),
		stats:            &priorityQueueStats{},
		starvationConfig: config,
		clock:            clock,
		stopCh:           make(chan struct{}),
	}

	// start starvation prevention goroutine
	if config.Enable {
		go pq.starvationGuard()
	}

	return pq
}

// Enqueue adds a task to the queue
func (pq *PriorityQueue) Enqueue(task *PriorityTask) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	heap.Push(&pq.heap, task)
	atomic.AddInt64(&pq.stats.totalEnqueued, 1)
	atomic.AddInt64(&pq.stats.currentSize, 1)
}

// Dequeue removes and returns the highest priority task
func (pq *PriorityQueue) Dequeue() (*PriorityTask, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.heap) == 0 {
		return nil, false
	}

	task := heap.Pop(&pq.heap).(*PriorityTask)
	atomic.AddInt64(&pq.stats.totalDequeued, 1)
	atomic.AddInt64(&pq.stats.currentSize, -1)

	// record wait time
	waitTime := pq.clock.Since(task.SubmitTime)
	atomic.AddInt64(&pq.stats.totalWaitTime, int64(waitTime))

	// update maximum wait time
	for {
		currentMax := atomic.LoadInt64(&pq.stats.maxWaitTime)
		if int64(waitTime) <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&pq.stats.maxWaitTime, currentMax, int64(waitTime)) {
			break
		}
	}

	return task, true
}

// Peek returns the highest priority task without removing it
func (pq *PriorityQueue) Peek() (*PriorityTask, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if len(pq.heap) == 0 {
		return nil, false
	}

	return pq.heap[0], true
}

// Size returns the queue size
func (pq *PriorityQueue) Size() int {
	return int(atomic.LoadInt64(&pq.stats.currentSize))
}

// IsEmpty checks if the queue is empty
func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Size() == 0
}

// GetStats gets priority queue statistics
func (pq *PriorityQueue) GetStats() PriorityStats {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	stats := PriorityStats{
		QueueLengthByPriority: make(map[int]int),
		TasksByPriority:       make(map[int]int64),
		CompletedByPriority:   make(map[int]int64),
	}

	// count tasks by priority in current queue
	for _, task := range pq.heap {
		effectivePriority := task.GetEffectivePriority()
		stats.QueueLengthByPriority[effectivePriority]++
		stats.TasksByPriority[effectivePriority]++
	}

	// get statistics
	totalDequeued := atomic.LoadInt64(&pq.stats.totalDequeued)
	totalWaitTime := atomic.LoadInt64(&pq.stats.totalWaitTime)

	if totalDequeued > 0 {
		stats.AverageWaitTime = time.Duration(totalWaitTime / totalDequeued)
	}

	stats.MaxWaitTime = time.Duration(atomic.LoadInt64(&pq.stats.maxWaitTime))
	stats.TotalWaitingTasks = atomic.LoadInt64(&pq.stats.currentSize)
	stats.StarvationDetected = atomic.LoadInt64(&pq.stats.starvationEvents)
	stats.AgePromotions = atomic.LoadInt64(&pq.stats.agePromotions)

	return stats
}

// Close closes the priority queue
func (pq *PriorityQueue) Close() {
	pq.stopOnce.Do(func() {
		close(pq.stopCh)
	})
}

// starvationGuard runs starvation prevention goroutine
func (pq *PriorityQueue) starvationGuard() {
	if !pq.starvationConfig.Enable {
		return
	}

	ticker := pq.clock.NewTicker(pq.starvationConfig.AgePromotionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C():
			pq.checkAndPromoteStarvedTasks()
		case <-pq.stopCh:
			return
		}
	}
}

// checkAndPromoteStarvedTasks checks and promotes starved tasks' priority
func (pq *PriorityQueue) checkAndPromoteStarvedTasks() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	now := pq.clock.Now()
	promoted := 0

	for _, task := range pq.heap {
		waitTime := now.Sub(task.SubmitTime)

		// check if starvation threshold is reached
		if waitTime > pq.starvationConfig.MaxWaitTime {
			// check if maximum boost is not yet reached
			if int(atomic.LoadInt32(&task.AgeBoost)) < pq.starvationConfig.MaxPriorityBoost {
				task.BoostPriority(pq.starvationConfig.PriorityBoostAmount)
				promoted++
				atomic.AddInt64(&pq.stats.agePromotions, 1)

				// record starvation event
				atomic.AddInt64(&pq.stats.starvationEvents, 1)
			}
		}
	}

	// if tasks were promoted, rebuild heap
	if promoted > 0 {
		heap.Init(&pq.heap)
	}
}

// UpdateTaskPriority dynamically updates task priority
func (pq *PriorityQueue) UpdateTaskPriority(taskID string, newPriority int) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// find task
	found := false
	for _, task := range pq.heap {
		if task.ID() == taskID {
			task.priority = newPriority
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task with ID %s not found in priority queue", taskID)
	}

	// rebuild heap to maintain priority order
	heap.Init(&pq.heap)

	return nil
}

// GetTasksByPriority gets tasks by priority level (for debugging and monitoring)
func (pq *PriorityQueue) GetTasksByPriority() map[int][]*PriorityTask {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make(map[int][]*PriorityTask)

	for _, task := range pq.heap {
		priority := task.GetEffectivePriority()
		result[priority] = append(result[priority], task)
	}

	return result
}

// WaitForCondition waits for queue to meet specific condition (for testing)
func (pq *PriorityQueue) WaitForCondition(ctx context.Context, condition func() bool, checkInterval time.Duration) error {
	ticker := pq.clock.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if condition() {
			return nil
		}

		select {
		case <-ticker.C():
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
