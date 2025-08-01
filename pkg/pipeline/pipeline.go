// Package pipeline provides core Pipeline implementation
package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// PipelineState defines the state of Pipeline
type PipelineState int32

const (
	// StateCreated Pipeline is created
	StateCreated PipelineState = iota
	// StateRunning Pipeline is running
	StateRunning
	// StateStopped Pipeline is stopped
	StateStopped
	// StateClosed Pipeline is closed
	StateClosed
)

// String returns string representation of state
func (s PipelineState) String() string {
	switch s {
	case StateCreated:
		return "Created"
	case StateRunning:
		return "Running"
	case StateStopped:
		return "Stopped"
	case StateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

// pipeline is a concrete implementation of Pipeline interface
// Note: this is a simplified implementation mainly for functional API support
// Recommend using functional APIs in functional.go and scalable_chain.go
type pipeline[T, R any] struct {
	// basic fields
	process ProcessFunc[T, R] // processing function
	config  *types.Config     // configuration
	state   int32             // atomic state, using int32 for atomic operations

	// concurrency control
	stopChan chan struct{} // stop signal
	once     sync.Once     // ensure Close executes only once

	// optional components
	retryPolicy  types.RetryPolicy  // retry policy
	errorHandler types.ErrorHandler // error handler
	workerPool   types.WorkerPool   // worker pool
}

// New creates a new Pipeline instance
func New[T, R any](process ProcessFunc[T, R], config *types.Config, opts ...types.Option[*pipeline[T, R]]) types.Pipeline[T, R] {
	// Validate process function
	if process == nil {
		panic("process function cannot be nil")
	}

	if config == nil {
		config = &types.Config{
			Workers:      1,
			BufferSize:   100,
			Timeout:      30 * time.Second,
			StageTimeout: 10 * time.Second,
			MaxRetries:   3,
			RetryDelay:   time.Second,
			Clock:        types.NewRealClock(),
		}
	}

	// Ensure clock is set
	if config.Clock == nil {
		config.Clock = types.NewRealClock()
	}

	p := &pipeline[T, R]{
		process:  process,
		config:   config,
		state:    int32(StateCreated),
		stopChan: make(chan struct{}),
	}

	// Apply configuration options
	for _, opt := range opts {
		opt(p)
	}

	return p
}

// GetClock returns the pipeline's clock
func (p *pipeline[T, R]) GetClock() types.Clock {
	return p.config.Clock
}

// Execute executes pipeline processing
func (p *pipeline[T, R]) Execute(ctx context.Context, input T) (R, error) {
	var zero R

	// Check state
	state := PipelineState(atomic.LoadInt32(&p.state))
	if state == StateClosed {
		return zero, types.ErrPipelineClosed
	}
	if state == StateStopped {
		return zero, types.ErrPipelineStopped
	}

	// Create context with timeout
	execCtx := ctx
	if p.config.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, p.config.Timeout)
		defer cancel()
	}

	// Execute processing function
	return p.executeWithRetry(execCtx, input)
}

// ExecuteAsync executes pipeline processing asynchronously
func (p *pipeline[T, R]) ExecuteAsync(ctx context.Context, input T) <-chan types.Result[R] {
	resultChan := make(chan types.Result[R], 1)

	go func() {
		defer close(resultChan)

		start := p.config.Clock.Now()
		result, err := p.Execute(ctx, input)
		duration := p.config.Clock.Since(start)

		// Use object pool to reduce allocations
		pooledResult := types.GetPooledResult[R]()
		pooledResult.Value = result
		pooledResult.Error = err
		pooledResult.Duration = duration

		resultChan <- *pooledResult
		// Return to pool after sending (note: the receiver gets a copy)
		types.PutPooledResult(pooledResult)
	}()

	return resultChan
}

// ExecuteBatch executes pipeline processing in batch
func (p *pipeline[T, R]) ExecuteBatch(ctx context.Context, inputs []T) <-chan types.BatchResult[R] {
	resultChan := make(chan types.BatchResult[R], len(inputs))

	go func() {
		defer close(resultChan)

		// Use worker pool for concurrent processing
		workers := p.config.Workers
		if workers <= 0 {
			workers = 1
		}

		// Create work channel
		workChan := make(chan struct {
			index int
			input T
		}, len(inputs))

		// Start workers
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for work := range workChan {
					start := p.config.Clock.Now()
					result, err := p.Execute(ctx, work.input)
					duration := p.config.Clock.Since(start)

					// Use object pool to reduce allocations
					pooledResult := types.GetPooledBatchResult[R]()
					pooledResult.Index = work.index
					pooledResult.Value = result
					pooledResult.Error = err
					pooledResult.Duration = duration

					select {
					case resultChan <- *pooledResult:
						// Return to pool after sending (receiver gets a copy)
						types.PutPooledBatchResult(pooledResult)
					case <-ctx.Done():
						// Return to pool on cancellation
						types.PutPooledBatchResult(pooledResult)
						return
					}
				}
			}()
		}

		// Send work tasks
		go func() {
			defer close(workChan)
			for i, input := range inputs {
				select {
				case workChan <- struct {
					index int
					input T
				}{index: i, input: input}:
				case <-ctx.Done():
					return
				}
			}
		}()

		wg.Wait()
	}()

	return resultChan
}

// Start starts the pipeline
func (p *pipeline[T, R]) Start() error {
	// Use atomic operation to check and update state
	if !atomic.CompareAndSwapInt32(&p.state, int32(StateCreated), int32(StateRunning)) {
		currentState := PipelineState(atomic.LoadInt32(&p.state))
		if currentState == StateRunning {
			return nil // Already running
		}
		return fmt.Errorf("cannot start pipeline in state %s", currentState)
	}

	return nil
}

// Stop stops the pipeline
func (p *pipeline[T, R]) Stop() error {
	// Use atomic operation to check and update state
	oldState := atomic.LoadInt32(&p.state)
	if PipelineState(oldState) == StateClosed {
		return types.ErrPipelineClosed
	}

	if atomic.CompareAndSwapInt32(&p.state, int32(StateRunning), int32(StateStopped)) ||
		atomic.CompareAndSwapInt32(&p.state, int32(StateCreated), int32(StateStopped)) {
		// Send stop signal
		select {
		case p.stopChan <- struct{}{}:
		default:
		}
	}

	return nil
}

// Close closes the pipeline and releases resources
func (p *pipeline[T, R]) Close() error {
	var err error
	p.once.Do(func() {
		// Update state to closed
		atomic.StoreInt32(&p.state, int32(StateClosed))

		// Close stop signal channel
		close(p.stopChan)
	})

	return err
}

// GetState gets the pipeline state
func (p *pipeline[T, R]) GetState() types.PipelineState {
	state := PipelineState(atomic.LoadInt32(&p.state))
	return types.PipelineState(state)
}

// executeWithRetry executes processing function with retry support
func (p *pipeline[T, R]) executeWithRetry(ctx context.Context, input T) (R, error) {
	var zero R
	var lastErr error

	maxRetries := p.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Execute processing function
		result, err := p.process(ctx, input)

		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if should retry
		if attempt >= maxRetries-1 {
			break
		}

		// Check retry policy
		if p.retryPolicy != nil && !p.retryPolicy.ShouldRetry(err, attempt) {
			break
		}

		// Wait for retry delay
		retryDelay := p.config.RetryDelay
		if p.retryPolicy != nil {
			retryDelay = p.retryPolicy.NextDelay(attempt)
		}

		if retryDelay > 0 {
			select {
			case <-p.config.Clock.After(retryDelay):
				// Continue to next retry attempt
			case <-ctx.Done():
				// Context cancellation has highest priority
				return zero, ctx.Err()
			case <-p.stopChan:
				// Check if context is also done to maintain consistency
				select {
				case <-ctx.Done():
					return zero, ctx.Err()
				default:
					return zero, types.ErrPipelineStopped
				}
			}
		}
	}

	// Apply error handler
	if p.errorHandler != nil {
		if handledErr := p.errorHandler(lastErr); handledErr != nil {
			lastErr = handledErr
		}
	}

	return zero, fmt.Errorf("pipeline failed after %d attempts: %w", maxRetries, lastErr)
}

// WithRetryPolicy sets retry policy
func WithRetryPolicy[T, R any](policy types.RetryPolicy) types.Option[*pipeline[T, R]] {
	return func(p *pipeline[T, R]) {
		p.retryPolicy = policy
	}
}

// WithErrorHandler sets error handler
func WithErrorHandler[T, R any](handler types.ErrorHandler) types.Option[*pipeline[T, R]] {
	return func(p *pipeline[T, R]) {
		p.errorHandler = handler
	}
}

// WithWorkerPool sets worker pool
func WithWorkerPool[T, R any](pool types.WorkerPool) types.Option[*pipeline[T, R]] {
	return func(p *pipeline[T, R]) {
		p.workerPool = pool
	}
}
