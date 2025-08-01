// Package retry provides retry executor implementation
package retry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// RetryExecutor implements retry execution logic
type RetryExecutor struct {
	policy         RetryPolicy
	eventHandler   EventHandler
	circuitBreaker CircuitBreaker
	stats          RetryStats
	clock          types.Clock
}

// ExecuteFunc is the function type to retry
type ExecuteFunc[T any] func(ctx context.Context) (T, error)

// RetryStats contains retry statistics
type RetryStats struct {
	TotalAttempts   int64         // total attempt count
	TotalRetries    int64         // total retry count
	TotalSuccesses  int64         // total success count
	TotalFailures   int64         // total failure count
	AverageAttempts float64       // average attempt count
	LastRetryTime   time.Time     // last retry time
	TotalRetryDelay time.Duration // total retry delay time
	mu              sync.RWMutex
}

// EventHandler handles retry events
type EventHandler interface {
	OnRetryAttempt(ctx context.Context, attempt int, err error)
	OnRetrySuccess(ctx context.Context, attempt int, duration time.Duration)
	OnRetryFailure(ctx context.Context, attempt int, err error)
	OnMaxAttemptsReached(ctx context.Context, attempt int, err error)
}

// CircuitBreaker interface (prepared for future circuit breaker integration)
type CircuitBreaker interface {
	Call(ctx context.Context, fn func() error) error
	IsOpen() bool
	Reset()
}

// NewRetryExecutor creates a retry executor
func NewRetryExecutor(policy RetryPolicy, opts ...ExecutorOption) *RetryExecutor {
	executor := &RetryExecutor{
		policy: policy,
		stats:  RetryStats{},
		clock:  types.NewRealClock(), // Default to real clock
	}

	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// Execute executes a function with retry logic
func Execute[T any](r *RetryExecutor, ctx context.Context, fn ExecuteFunc[T]) (T, error) {
	return ExecuteWithName(r, ctx, "default", fn)
}

// ExecuteWithName executes a function with retry logic (with name for metrics and events)
func ExecuteWithName[T any](r *RetryExecutor, ctx context.Context, name string, fn ExecuteFunc[T]) (T, error) {
	var zero T
	attempt := 0

	// reset policy state
	r.policy.Reset()

	for {
		attempt++

		// update statistics
		r.updateStats(func(stats *RetryStats) {
			stats.TotalAttempts++
		})

		// check if context is cancelled
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// check circuit breaker state
		if r.circuitBreaker != nil && r.circuitBreaker.IsOpen() {
			return zero, fmt.Errorf("circuit breaker is open")
		}

		// trigger retry attempt event
		if r.eventHandler != nil && attempt > 1 {
			r.eventHandler.OnRetryAttempt(ctx, attempt, nil)
		}

		// execute function
		executeStart := r.clock.Now()
		result, err := fn(ctx)
		executeDuration := r.clock.Since(executeStart)

		// execution successful
		if err == nil {
			r.updateStats(func(stats *RetryStats) {
				stats.TotalSuccesses++
				if attempt > 1 {
					stats.TotalRetries++
				}
				stats.updateAverageAttempts(attempt)
			})

			if r.eventHandler != nil && attempt > 1 {
				r.eventHandler.OnRetrySuccess(ctx, attempt, executeDuration)
			}

			return result, nil
		}

		// check if should retry
		if !r.policy.ShouldRetry(err, attempt) {
			// reached max attempts or should not retry
			r.updateStats(func(stats *RetryStats) {
				stats.TotalFailures++
				if attempt > 1 {
					stats.TotalRetries++
				}
				stats.updateAverageAttempts(attempt)
			})

			if r.eventHandler != nil {
				if attempt >= r.policy.MaxAttempts() {
					r.eventHandler.OnMaxAttemptsReached(ctx, attempt, err)
				} else {
					r.eventHandler.OnRetryFailure(ctx, attempt, err)
				}
			}

			return zero, r.wrapError(err, attempt)
		}

		// calculate delay time
		delay := r.policy.NextDelay(attempt)

		// update statistics
		r.updateStats(func(stats *RetryStats) {
			stats.LastRetryTime = r.clock.Now()
			stats.TotalRetryDelay += delay
		})

		// wait for retry delay
		if delay > 0 {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-r.clock.After(delay):
				// continue retrying
			}
		}
	}
}

// ExecuteAsync executes a function with retry asynchronously
func ExecuteAsync[T any](r *RetryExecutor, ctx context.Context, fn ExecuteFunc[T]) <-chan types.Result[T] {
	return ExecuteAsyncWithName(r, ctx, "default", fn)
}

// ExecuteAsyncWithName executes a function with retry asynchronously (with name)
func ExecuteAsyncWithName[T any](r *RetryExecutor, ctx context.Context, name string, fn ExecuteFunc[T]) <-chan types.Result[T] {
	resultChan := make(chan types.Result[T], 1)

	go func() {
		defer close(resultChan)

		start := r.clock.Now()
		value, err := ExecuteWithName(r, ctx, name, fn)
		duration := r.clock.Since(start)

		resultChan <- types.Result[T]{
			Value:    value,
			Error:    err,
			Duration: duration,
		}
	}()

	return resultChan
}

// GetStats gets retry statistics
func (r *RetryExecutor) GetStats() RetryStats {
	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()
	return RetryStats{
		TotalAttempts:   r.stats.TotalAttempts,
		TotalRetries:    r.stats.TotalRetries,
		TotalSuccesses:  r.stats.TotalSuccesses,
		TotalFailures:   r.stats.TotalFailures,
		AverageAttempts: r.stats.AverageAttempts,
		LastRetryTime:   r.stats.LastRetryTime,
		TotalRetryDelay: r.stats.TotalRetryDelay,
		// don't copy mutex
	}
}

// ResetStats resets statistics
func (r *RetryExecutor) ResetStats() {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()

	// reset all fields but keep mutex
	r.stats.TotalAttempts = 0
	r.stats.TotalRetries = 0
	r.stats.TotalSuccesses = 0
	r.stats.TotalFailures = 0
	r.stats.AverageAttempts = 0
	r.stats.LastRetryTime = time.Time{}
	r.stats.TotalRetryDelay = 0
}

// updateStats updates statistics (thread-safe)
func (r *RetryExecutor) updateStats(fn func(*RetryStats)) {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	fn(&r.stats)
}

// updateAverageAttempts updates average attempt count
func (s *RetryStats) updateAverageAttempts(attempt int) {
	totalOperations := s.TotalSuccesses + s.TotalFailures
	if totalOperations > 0 {
		s.AverageAttempts = float64(s.TotalAttempts) / float64(totalOperations)
	}
}

// wrapError wraps error with retry information
func (r *RetryExecutor) wrapError(err error, attempts int) error {
	if pipelineErr, ok := err.(*types.PipelineError); ok {
		pipelineErr.WithContext("retry_attempts", attempts)
		pipelineErr.WithContext("max_attempts", r.policy.MaxAttempts())
		return pipelineErr
	}

	// create new Pipeline error
	retryErr := types.NewPipelineError("retry", nil, err)
	retryErr.WithContext("retry_attempts", attempts)
	retryErr.WithContext("max_attempts", r.policy.MaxAttempts())

	return retryErr
}

// ExecutorOption is a configuration option for retry executor
type ExecutorOption func(*RetryExecutor)

// WithEventHandler sets the event handler
func WithEventHandler(handler EventHandler) ExecutorOption {
	return func(r *RetryExecutor) {
		r.eventHandler = handler
	}
}

// WithCircuitBreaker sets the circuit breaker
func WithCircuitBreaker(breaker CircuitBreaker) ExecutorOption {
	return func(r *RetryExecutor) {
		r.circuitBreaker = breaker
	}
}

// WithClock sets the clock for time operations
func WithClock(clock types.Clock) ExecutorOption {
	return func(r *RetryExecutor) {
		r.clock = clock
	}
}

// DefaultEventHandler is the default event handler implementation
type DefaultEventHandler struct {
	logger Logger
}

// Logger interface for logging
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// NewDefaultEventHandler creates a default event handler
func NewDefaultEventHandler(logger Logger) *DefaultEventHandler {
	return &DefaultEventHandler{logger: logger}
}

// OnRetryAttempt handles retry attempt events
func (h *DefaultEventHandler) OnRetryAttempt(ctx context.Context, attempt int, err error) {
	if h.logger != nil {
		h.logger.Debugf("Retry attempt %d starting", attempt)
	}
}

// OnRetrySuccess handles retry success events
func (h *DefaultEventHandler) OnRetrySuccess(ctx context.Context, attempt int, duration time.Duration) {
	if h.logger != nil {
		h.logger.Infof("Retry succeeded on attempt %d after %v", attempt, duration)
	}
}

// OnRetryFailure handles retry failure events
func (h *DefaultEventHandler) OnRetryFailure(ctx context.Context, attempt int, err error) {
	if h.logger != nil {
		h.logger.Warnf("Retry attempt %d failed: %v", attempt, err)
	}
}

// OnMaxAttemptsReached handles max attempts reached events
func (h *DefaultEventHandler) OnMaxAttemptsReached(ctx context.Context, attempt int, err error) {
	if h.logger != nil {
		h.logger.Errorf("Max retry attempts (%d) reached, final error: %v", attempt, err)
	}
}
