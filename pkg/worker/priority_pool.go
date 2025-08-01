// Package worker provides priority worker pool implementation
package worker

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// PriorityWorkerPoolConfig contains configuration for priority worker pool
type PriorityWorkerPoolConfig struct {
	// PoolSize is the number of workers in the pool
	PoolSize int

	// QueueCapacity is the queue capacity (-1 for unlimited)
	QueueCapacity int

	// SubmitTimeout is the timeout for task submission
	SubmitTimeout time.Duration

	// Clock for time operations (optional, defaults to real clock)
	Clock types.Clock

	// ErrorHandler handles worker errors
	ErrorHandler types.ErrorHandler

	// StarvationConfig prevents task starvation
	StarvationConfig *StarvationConfig

	// PriorityLevels is the number of supported priority levels
	PriorityLevels int

	// DefaultPriority is the default task priority
	DefaultPriority int
}

// DefaultPriorityWorkerPoolConfig returns default configuration
func DefaultPriorityWorkerPoolConfig() *PriorityWorkerPoolConfig {
	return &PriorityWorkerPoolConfig{
		PoolSize:         10,
		QueueCapacity:    1000,
		SubmitTimeout:    5 * time.Second,
		Clock:            types.NewRealClock(),
		StarvationConfig: DefaultStarvationConfig(),
		PriorityLevels:   10,
		DefaultPriority:  5,
	}
}

// PriorityWorkerPool implements priority-based worker pool
type PriorityWorkerPool struct {
	config        *PriorityWorkerPoolConfig
	workers       []*Worker
	priorityQueue *PriorityQueue
	taskChan      chan *PriorityTask

	// State management
	state     int32 // 0: stopped, 1: running, 2: closed
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	// Synchronization
	mu sync.RWMutex

	// Condition variable to notify workers of new tasks
	taskNotify *sync.Cond
}

// NewPriorityWorkerPool creates a new priority worker pool
func NewPriorityWorkerPool(config *PriorityWorkerPoolConfig) (*PriorityWorkerPool, error) {
	if config == nil {
		config = DefaultPriorityWorkerPoolConfig()
	}

	// Validate parameters
	if config.PoolSize <= 0 {
		return nil, fmt.Errorf("pool size must be positive, got %d", config.PoolSize)
	}
	if config.QueueCapacity < -1 || config.QueueCapacity == 0 {
		return nil, fmt.Errorf("queue capacity must be positive or -1 for unlimited, got %d", config.QueueCapacity)
	}

	// Ensure clock is set
	if config.Clock == nil {
		config.Clock = types.NewRealClock()
	}

	// Create task channel
	var taskChan chan *PriorityTask
	if config.QueueCapacity > 0 {
		taskChan = make(chan *PriorityTask, config.QueueCapacity)
	} else {
		taskChan = make(chan *PriorityTask)
	}

	// Create priority queue
	priorityQueue := NewPriorityQueueWithClock(config.StarvationConfig, config.Clock)

	pool := &PriorityWorkerPool{
		config:        config,
		workers:       make([]*Worker, config.PoolSize),
		priorityQueue: priorityQueue,
		taskChan:      taskChan,
	}

	// Initialize condition variable
	pool.taskNotify = sync.NewCond(&pool.mu)

	// Create workers
	for i := 0; i < config.PoolSize; i++ {
		worker := NewWorkerWithClock(i, nil, config.Clock) // No traditional task channel
		if config.ErrorHandler != nil {
			worker.SetErrorHandler(config.ErrorHandler)
		}
		// Set completion callback
		pool.workers[i] = worker
	}

	// Task scheduler will start on Start()

	return pool, nil
}

// Start starts the worker pool
func (p *PriorityWorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.state, 0, 1) {
		state := atomic.LoadInt32(&p.state)
		if state == 1 {
			return fmt.Errorf("priority worker pool is already running")
		}
		return fmt.Errorf("priority worker pool is closed")
	}

	// Create context
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start task scheduler
	go p.taskScheduler()

	// Start all workers with custom task fetching logic
	for _, worker := range p.workers {
		go p.runPriorityWorker(worker)
	}

	return nil
}

// Submit submits a task to the worker pool with default priority
func (p *PriorityWorkerPool) Submit(task types.Task) error {
	return p.SubmitWithPriority(task, p.config.DefaultPriority)
}

// SubmitWithTimeout submits a task with timeout using default priority
func (p *PriorityWorkerPool) SubmitWithTimeout(task types.Task, timeout time.Duration) error {
	return p.SubmitWithPriorityAndTimeout(task, p.config.DefaultPriority, timeout)
}

// SubmitWithPriority submits a task with specified priority
func (p *PriorityWorkerPool) SubmitWithPriority(task types.Task, priority int) error {
	return p.SubmitWithPriorityAndTimeout(task, priority, p.config.SubmitTimeout)
}

// SubmitWithPriorityAndTimeout submits a task with priority and timeout
func (p *PriorityWorkerPool) SubmitWithPriorityAndTimeout(task types.Task, priority int, timeout time.Duration) error {
	// Check pool state
	state := atomic.LoadInt32(&p.state)
	if state != 1 {
		if state == 0 {
			return fmt.Errorf("priority worker pool is not started")
		}
		return fmt.Errorf("priority worker pool is closed")
	}

	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Create priority task
	priorityTask := NewPriorityTaskWithClock(task, priority, p.config.Clock)

	// If no timeout, try to send directly
	if timeout <= 0 {
		select {
		case p.taskChan <- priorityTask:
			return nil
		default:
			return types.ErrWorkerPoolFull
		}
	}

	// Task submission with timeout
	timer := p.config.Clock.NewTimer(timeout)
	defer timer.Stop()

	select {
	case p.taskChan <- priorityTask:
		return nil
	case <-timer.C():
		return types.ErrTimeout
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// UpdatePriority dynamically updates task priority
func (p *PriorityWorkerPool) UpdatePriority(taskID string, priority int) error {
	return p.priorityQueue.UpdateTaskPriority(taskID, priority)
}

// GetPriorityStats returns priority statistics
func (p *PriorityWorkerPool) GetPriorityStats() PriorityStats {
	return p.priorityQueue.GetStats()
}

// Stop stops the worker pool
func (p *PriorityWorkerPool) Stop() error {
	if !atomic.CompareAndSwapInt32(&p.state, 1, 0) {
		state := atomic.LoadInt32(&p.state)
		if state == 0 {
			return fmt.Errorf("priority worker pool is not running")
		}
		return fmt.Errorf("priority worker pool is closed")
	}

	// Cancel context to notify all workers to stop
	if p.cancel != nil {
		p.cancel()
	}

	// Notify all waiting workers
	p.mu.Lock()
	p.taskNotify.Broadcast()
	p.mu.Unlock()

	// Wait for all workers to stop
	var wg sync.WaitGroup
	for _, worker := range p.workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			if err := w.Stop(); err != nil {
				// Log but don't return error
			}
		}(worker)
	}

	// Wait for all workers to stop with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers stopped normally
		return nil
	case <-p.config.Clock.After(2 * time.Second): // Reduced timeout
		// Timeout, but allow graceful failure
		return nil
	}
}

// Close closes the worker pool and releases resources
func (p *PriorityWorkerPool) Close() error {
	var closeErr error

	p.closeOnce.Do(func() {
		// Try to stop pool, ignore "not running" errors
		if err := p.Stop(); err != nil && !strings.Contains(err.Error(), "not running") {
			closeErr = err
			return
		}

		// Set to closed state
		atomic.StoreInt32(&p.state, 2)

		// Close priority queue
		p.priorityQueue.Close()

		// Close task channel
		close(p.taskChan)

		// Note: Don't set p.taskChan and p.priorityQueue to nil
		// as other goroutines might still be accessing these fields
	})

	return closeErr
}

// Size returns the worker pool size
func (p *PriorityWorkerPool) Size() int {
	return p.config.PoolSize
}

// Stats returns basic worker pool statistics
func (p *PriorityWorkerPool) Stats() types.WorkerPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Count active workers
	var activeWorkers int
	for _, worker := range p.workers {
		if worker.State() == WorkerStateWorking {
			activeWorkers++
		}
	}

	queueCapacity := p.config.QueueCapacity
	if queueCapacity == -1 {
		queueCapacity = 0 // Unlimited shown as 0
	}

	return types.WorkerPoolStats{
		PoolSize:      p.config.PoolSize,
		ActiveWorkers: activeWorkers,
		QueueSize:     p.priorityQueue.Size() + len(p.taskChan),
		QueueCapacity: queueCapacity,
	}
}

// taskScheduler reads tasks from channel and enqueues them in priority queue
func (p *PriorityWorkerPool) taskScheduler() {
	for {
		select {
		case task := <-p.taskChan:
			if task == nil {
				return
			}

			// Enqueue task to priority queue
			p.priorityQueue.Enqueue(task)

			// Notify waiting workers
			p.mu.Lock()
			p.taskNotify.Signal()
			p.mu.Unlock()

		case <-p.ctx.Done():
			return
		}
	}
}

// runPriorityWorker runs a priority worker
func (p *PriorityWorkerPool) runPriorityWorker(worker *Worker) {
	// Ensure done channel is closed on exit for Worker.Stop() to work
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash the program
		}
		// Set worker state to stopped and close done channel
		worker.SetState(WorkerStateStopped)
		worker.CloseDone()
	}()

	// Set worker to idle state
	worker.SetState(WorkerStateIdle)

	for {
		// Check if should stop
		select {
		case <-p.ctx.Done():
			return
		case <-worker.QuitChannel():
			return
		default:
		}

		// Get next task from priority queue
		task := p.getNextTask()
		if task == nil {
			// No task, check if should stop
			if atomic.LoadInt32(&p.state) != 1 {
				return
			}
			continue
		}

		// Set to working state
		worker.SetState(WorkerStateWorking)

		// Execute task
		err := task.Execute(p.ctx)

		// Set back to idle state
		worker.SetState(WorkerStateIdle)

		// Handle error if error handler exists
		if err != nil && p.config.ErrorHandler != nil {
			p.config.ErrorHandler(err)
		}
	}
}

// getNextTask gets the next task from priority queue
func (p *PriorityWorkerPool) getNextTask() *PriorityTask {
	for {
		p.mu.Lock()

		// Try to get task from priority queue
		if task, ok := p.priorityQueue.Dequeue(); ok {
			p.mu.Unlock()
			return task
		}

		// Check if should stop
		if atomic.LoadInt32(&p.state) != 1 {
			p.mu.Unlock()
			return nil
		}

		// No task, wait using condition variable
		// Check if context is cancelled
		select {
		case <-p.ctx.Done():
			p.mu.Unlock()
			return nil
		default:
		}

		// Wait for task notification (automatically releases and reacquires lock)
		p.taskNotify.Wait()
		// Wait() returns with lock reacquired, but we need to unlock
		// before next loop iteration acquires lock again
		p.mu.Unlock()

		// Give other goroutines a chance to run
		runtime.Gosched()
	}
}

// IsRunning checks if the worker pool is running
func (p *PriorityWorkerPool) IsRunning() bool {
	return atomic.LoadInt32(&p.state) == 1
}

// IsClosed checks if the worker pool is closed
func (p *PriorityWorkerPool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == 2
}

// QueueLength returns current queue length (priority queue + buffered channel)
func (p *PriorityWorkerPool) QueueLength() int {
	return p.priorityQueue.Size() + len(p.taskChan)
}

// QueueCapacity returns the queue capacity
func (p *PriorityWorkerPool) QueueCapacity() int {
	return p.config.QueueCapacity
}

// GetTasksByPriority returns tasks grouped by priority (for debugging and monitoring)
func (p *PriorityWorkerPool) GetTasksByPriority() map[int][]*PriorityTask {
	return p.priorityQueue.GetTasksByPriority()
}
