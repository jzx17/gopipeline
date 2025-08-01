package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// DynamicWorkerPoolConfig contains configuration for dynamic worker pool
type DynamicWorkerPoolConfig struct {
	// MinWorkers is the minimum number of workers
	MinWorkers int

	// MaxWorkers is the maximum number of workers
	MaxWorkers int

	// IdleTimeout is the worker idle timeout duration
	IdleTimeout time.Duration

	// QueueSize is the task queue size
	QueueSize int

	// SubmitTimeout is the task submission timeout
	SubmitTimeout time.Duration

	// Clock for time operations (optional, defaults to real clock)
	Clock types.Clock

	// ErrorHandler handles worker errors
	ErrorHandler types.ErrorHandler
}

// DefaultDynamicWorkerPoolConfig returns default configuration
func DefaultDynamicWorkerPoolConfig() *DynamicWorkerPoolConfig {
	return &DynamicWorkerPoolConfig{
		MinWorkers:    2,
		MaxWorkers:    runtime.NumCPU() * 2,
		IdleTimeout:   30 * time.Second,
		QueueSize:     100,
		SubmitTimeout: 5 * time.Second,
		Clock:         types.NewRealClock(),
	}
}

// DynamicWorkerPool implements dynamic worker pool with auto-scaling
type DynamicWorkerPool struct {
	config   *DynamicWorkerPoolConfig
	workers  []*Worker
	taskChan chan types.Task

	// State management
	state     int32 // 0: stopped, 1: running, 2: closed
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	// Synchronization
	mu sync.RWMutex

	// Worker management
	nextWorkerID int32 // Next worker ID
}

// NewDynamicWorkerPool creates a new dynamic worker pool
func NewDynamicWorkerPool(config *DynamicWorkerPoolConfig) (*DynamicWorkerPool, error) {
	if config == nil {
		config = DefaultDynamicWorkerPoolConfig()
	}

	// Validate parameters
	if config.MinWorkers <= 0 {
		return nil, fmt.Errorf("min workers must be positive, got %d", config.MinWorkers)
	}
	if config.MaxWorkers < config.MinWorkers {
		return nil, fmt.Errorf("max workers (%d) must be >= min workers (%d)",
			config.MaxWorkers, config.MinWorkers)
	}
	if config.QueueSize <= 0 {
		return nil, fmt.Errorf("queue size must be positive, got %d", config.QueueSize)
	}

	// Ensure clock is set
	if config.Clock == nil {
		config.Clock = types.NewRealClock()
	}

	taskChan := make(chan types.Task, config.QueueSize)

	pool := &DynamicWorkerPool{
		config:       config,
		workers:      make([]*Worker, 0, config.MaxWorkers),
		taskChan:     taskChan,
		nextWorkerID: 0,
	}

	// Create minimum number of workers
	for i := 0; i < config.MinWorkers; i++ {
		worker := pool.createWorker()
		pool.workers = append(pool.workers, worker)
	}

	return pool, nil
}

// createWorker creates a new worker
func (p *DynamicWorkerPool) createWorker() *Worker {
	id := atomic.AddInt32(&p.nextWorkerID, 1) - 1
	worker := NewWorkerWithClock(int(id), p.taskChan, p.config.Clock)

	if p.config.ErrorHandler != nil {
		worker.SetErrorHandler(p.config.ErrorHandler)
	}

	return worker
}

// Start starts the worker pool
func (p *DynamicWorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.state, 0, 1) {
		state := atomic.LoadInt32(&p.state)
		if state == 1 {
			return fmt.Errorf("dynamic worker pool is already running")
		}
		return fmt.Errorf("dynamic worker pool is closed")
	}

	// Create context
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start all workers
	p.mu.Lock()
	for _, worker := range p.workers {
		go worker.Start(p.ctx)
	}
	p.mu.Unlock()

	return nil
}

// Submit submits a task to the worker pool
func (p *DynamicWorkerPool) Submit(task types.Task) error {
	return p.SubmitWithTimeout(task, p.config.SubmitTimeout)
}

// SubmitWithTimeout submits a task with timeout
func (p *DynamicWorkerPool) SubmitWithTimeout(task types.Task, timeout time.Duration) error {
	// Check pool state
	state := atomic.LoadInt32(&p.state)
	if state != 1 {
		if state == 0 {
			return fmt.Errorf("dynamic worker pool is not started")
		}
		return fmt.Errorf("dynamic worker pool is closed")
	}

	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// If no timeout, try to send directly
	if timeout <= 0 {
		select {
		case p.taskChan <- task:
			return nil
		default:
			return types.ErrWorkerPoolFull
		}
	}

	// Task submission with timeout
	timer := p.config.Clock.NewTimer(timeout)
	defer timer.Stop()

	select {
	case p.taskChan <- task:
		return nil
	case <-timer.C():
		return types.ErrTimeout
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Stop stops the worker pool
func (p *DynamicWorkerPool) Stop() error {
	if !atomic.CompareAndSwapInt32(&p.state, 1, 0) {
		state := atomic.LoadInt32(&p.state)
		if state == 0 {
			return nil // Already stopped, no error
		}
		return fmt.Errorf("dynamic worker pool is closed")
	}

	// Cancel context to notify all workers to stop
	if p.cancel != nil {
		p.cancel()
	}

	// Wait for all workers to stop
	p.mu.Lock()
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
	p.mu.Unlock()

	// Wait for all workers to stop with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-p.config.Clock.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for workers to stop")
	}
}

// Close closes the worker pool and releases resources
func (p *DynamicWorkerPool) Close() error {
	var closeErr error

	p.closeOnce.Do(func() {
		// Stop pool first
		if err := p.Stop(); err != nil {
			closeErr = err
			return
		}

		// Set to closed state
		atomic.StoreInt32(&p.state, 2)

		// Close task channel
		close(p.taskChan)

		// Clean up resources
		p.mu.Lock()
		p.workers = nil
		p.taskChan = nil
		p.mu.Unlock()
	})

	return closeErr
}

// Size returns the worker pool size
func (p *DynamicWorkerPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// GetMinWorkers returns the minimum number of workers
func (p *DynamicWorkerPool) GetMinWorkers() int {
	return p.config.MinWorkers
}

// GetMaxWorkers returns the maximum number of workers
func (p *DynamicWorkerPool) GetMaxWorkers() int {
	return p.config.MaxWorkers
}

// GetCurrentWorkers returns the current number of workers
func (p *DynamicWorkerPool) GetCurrentWorkers() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// ScaleUp scales up to the specified number of workers
func (p *DynamicWorkerPool) ScaleUp(targetSize int) error {
	if targetSize > p.config.MaxWorkers {
		return fmt.Errorf("target size %d exceeds max workers %d",
			targetSize, p.config.MaxWorkers)
	}

	p.mu.Lock()

	currentSize := len(p.workers)
	if targetSize <= currentSize {
		p.mu.Unlock()
		return fmt.Errorf("target size %d is not greater than current size %d",
			targetSize, currentSize)
	}

	// Create new workers (within lock)
	var newWorkers []*Worker
	var startCtx context.Context
	shouldStart := false
	
	// Check state and capture context atomically
	state := atomic.LoadInt32(&p.state)
	if state == 1 && p.ctx != nil {
		shouldStart = true
		startCtx = p.ctx
	}
	
	for i := currentSize; i < targetSize; i++ {
		worker := p.createWorker()
		p.workers = append(p.workers, worker)

		// If pool should be running, collect workers that need to be started
		if shouldStart {
			newWorkers = append(newWorkers, worker)
		}
	}

	// Release lock
	p.mu.Unlock()

	// Start new workers outside lock to avoid deadlock
	// Re-check state before starting to handle race conditions
	if shouldStart && atomic.LoadInt32(&p.state) == 1 {
		for _, worker := range newWorkers {
			// Use captured context to avoid nil pointer dereference
			go worker.Start(startCtx)
		}
	}

	return nil
}

// ScaleDown scales down to the specified number of workers
func (p *DynamicWorkerPool) ScaleDown(targetSize int) error {
	if targetSize < p.config.MinWorkers {
		return fmt.Errorf("target size %d is less than min workers %d",
			targetSize, p.config.MinWorkers)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	currentSize := len(p.workers)
	if targetSize >= currentSize {
		return fmt.Errorf("target size %d is not less than current size %d",
			targetSize, currentSize)
	}

	// Stop excess workers
	workersToRemove := currentSize - targetSize
	var wg sync.WaitGroup

	for i := 0; i < workersToRemove && len(p.workers) > targetSize; i++ {
		// Remove worker from the end
		workerIndex := len(p.workers) - 1
		worker := p.workers[workerIndex]
		p.workers = p.workers[:workerIndex]

		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			if err := w.Stop(); err != nil {
				// Log but don't affect other workers' stopping
			}
		}(worker)
	}

	// Wait for workers to stop in background with timeout to prevent goroutine leak
	go func() {
		done := make(chan struct{})
		go func() {
			defer close(done) // Ensure channel is always closed
			wg.Wait()
		}()

		// Set timeout mechanism to prevent goroutine leak
		select {
		case <-done:
			// All workers stopped normally
		case <-p.config.Clock.After(30 * time.Second):
			// Timeout, log warning but don't block main flow
			// The waiting goroutine will still complete when wg.Wait() finishes
			// and will be cleaned up properly due to the defer close(done)
		}
	}()

	return nil
}

// Stats returns basic worker pool statistics
func (p *DynamicWorkerPool) Stats() types.WorkerPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Count active workers
	var activeWorkers int
	for _, worker := range p.workers {
		if worker.State() == WorkerStateWorking {
			activeWorkers++
		}
	}

	return types.WorkerPoolStats{
		PoolSize:      len(p.workers),
		ActiveWorkers: activeWorkers,
		QueueSize:     len(p.taskChan),
		QueueCapacity: p.config.QueueSize,
	}
}

// IsRunning checks if the worker pool is running
func (p *DynamicWorkerPool) IsRunning() bool {
	return atomic.LoadInt32(&p.state) == 1
}

// IsClosed checks if the worker pool is closed
func (p *DynamicWorkerPool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == 2
}

// GetWorkerStats returns statistics for all workers
func (p *DynamicWorkerPool) GetWorkerStats() []WorkerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]WorkerStats, len(p.workers))
	for i, worker := range p.workers {
		stats[i] = worker.Stats()
	}
	return stats
}
