package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// FixedWorkerPoolConfig defines configuration for fixed worker pool
type FixedWorkerPoolConfig struct {
	// PoolSize is the size of the worker pool
	PoolSize int

	// QueueSize is the task queue size
	QueueSize int

	// SubmitTimeout is the task submission timeout
	SubmitTimeout time.Duration

	// Clock for time operations (optional, defaults to real clock)
	Clock types.Clock

	// ErrorHandler is the error handler
	ErrorHandler types.ErrorHandler
}

// DefaultFixedWorkerPoolConfig returns default configuration
func DefaultFixedWorkerPoolConfig() *FixedWorkerPoolConfig {
	return &FixedWorkerPoolConfig{
		PoolSize:      10,
		QueueSize:     100,
		SubmitTimeout: 5 * time.Second,
		Clock:         types.NewRealClock(),
	}
}

// FixedWorkerPool implements a fixed-size worker pool
type FixedWorkerPool struct {
	config   *FixedWorkerPoolConfig
	workers  []*Worker
	taskChan chan types.Task

	// state management
	state     int32 // 0: stopped, 1: running, 2: closed
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	// synchronization
	mu sync.RWMutex
}

// NewFixedWorkerPool creates a new fixed worker pool
func NewFixedWorkerPool(config *FixedWorkerPoolConfig) (*FixedWorkerPool, error) {
	if config == nil {
		config = DefaultFixedWorkerPoolConfig()
	}

	// parameter validation
	if config.PoolSize <= 0 {
		return nil, fmt.Errorf("pool size must be positive, got %d", config.PoolSize)
	}
	if config.QueueSize <= 0 {
		return nil, fmt.Errorf("queue size must be positive, got %d", config.QueueSize)
	}

	// Ensure clock is set
	if config.Clock == nil {
		config.Clock = types.NewRealClock()
	}

	taskChan := make(chan types.Task, config.QueueSize)
	workers := make([]*Worker, config.PoolSize)

	pool := &FixedWorkerPool{
		config:   config,
		workers:  workers,
		taskChan: taskChan,
	}

	// create workers
	for i := 0; i < config.PoolSize; i++ {
		worker := NewWorkerWithClock(i, taskChan, config.Clock)
		if config.ErrorHandler != nil {
			worker.SetErrorHandler(config.ErrorHandler)
		}
		workers[i] = worker
	}

	return pool, nil
}

// Start starts the worker pool
func (p *FixedWorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.state, 0, 1) {
		state := atomic.LoadInt32(&p.state)
		if state == 1 {
			return fmt.Errorf("worker pool is already running")
		}
		return fmt.Errorf("worker pool is closed")
	}

	// create context
	p.ctx, p.cancel = context.WithCancel(ctx)

	// start all workers
	for _, worker := range p.workers {
		go worker.Start(p.ctx)
	}

	return nil
}

// Submit submits a task to the worker pool
func (p *FixedWorkerPool) Submit(task types.Task) error {
	return p.SubmitWithTimeout(task, p.config.SubmitTimeout)
}

// SubmitWithTimeout submits a task to the worker pool with timeout
func (p *FixedWorkerPool) SubmitWithTimeout(task types.Task, timeout time.Duration) error {
	// check pool state
	state := atomic.LoadInt32(&p.state)
	if state != 1 {
		if state == 0 {
			return fmt.Errorf("worker pool is not started")
		}
		return fmt.Errorf("worker pool is closed")
	}

	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// if no timeout, try to send directly
	if timeout <= 0 {
		select {
		case p.taskChan <- task:
			return nil
		default:
			return types.ErrWorkerPoolFull
		}
	}

	// task submission with timeout
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
func (p *FixedWorkerPool) Stop() error {
	if !atomic.CompareAndSwapInt32(&p.state, 1, 0) {
		state := atomic.LoadInt32(&p.state)
		if state == 0 {
			return fmt.Errorf("worker pool is not running")
		}
		return fmt.Errorf("worker pool is closed")
	}

	// cancel context to notify all workers to stop
	if p.cancel != nil {
		p.cancel()
	}

	// wait for all workers to stop
	var wg sync.WaitGroup
	for _, worker := range p.workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			if err := w.Stop(); err != nil {
				// log but don't return error
			}
		}(worker)
	}

	// wait for all workers to stop with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// all workers stopped normally
		return nil
	case <-p.config.Clock.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for workers to stop")
	}
}

// Close closes the worker pool and releases resources
func (p *FixedWorkerPool) Close() error {
	var closeErr error

	p.closeOnce.Do(func() {
		// stop the pool first
		if err := p.Stop(); err != nil {
			closeErr = err
			return
		}

		// set to closed state
		atomic.StoreInt32(&p.state, 2)

		// close task channel
		close(p.taskChan)

		// clean up resources
		p.workers = nil
		p.taskChan = nil
	})

	return closeErr
}

// Size returns the worker pool size
func (p *FixedWorkerPool) Size() int {
	return p.config.PoolSize
}

// Stats gets basic worker pool statistics
func (p *FixedWorkerPool) Stats() types.WorkerPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// count active workers
	var activeWorkers int
	for _, worker := range p.workers {
		if worker.State() == WorkerStateWorking {
			activeWorkers++
		}
	}

	return types.WorkerPoolStats{
		PoolSize:      p.config.PoolSize,
		ActiveWorkers: activeWorkers,
		QueueSize:     len(p.taskChan),
		QueueCapacity: p.config.QueueSize,
	}
}

// GetWorkerStats gets statistics of all Workers
func (p *FixedWorkerPool) GetWorkerStats() []WorkerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]WorkerStats, len(p.workers))
	for i, worker := range p.workers {
		stats[i] = worker.Stats()
	}
	return stats
}

// IsRunning checks if the worker pool is running
func (p *FixedWorkerPool) IsRunning() bool {
	return atomic.LoadInt32(&p.state) == 1
}

// IsClosed checks if the worker pool is closed
func (p *FixedWorkerPool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == 2
}

// QueueLength gets the current queue length
func (p *FixedWorkerPool) QueueLength() int {
	if p.taskChan == nil {
		return 0
	}
	return len(p.taskChan)
}

// QueueCapacity gets the queue capacity
func (p *FixedWorkerPool) QueueCapacity() int {
	return p.config.QueueSize
}
