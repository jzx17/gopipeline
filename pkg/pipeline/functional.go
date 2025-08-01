// Package pipeline provides functional Pipeline implementation
package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// ProcessFunc defines the processing function type
type ProcessFunc[T, R any] func(context.Context, T) (R, error)

// Config simplified configuration structure
type Config struct {
	Workers      int
	BufferSize   int
	Timeout      time.Duration
	StageTimeout time.Duration
	MaxRetries   int
	RetryDelay   time.Duration
	Clock        types.Clock
}

// FunctionalPipeline functional Pipeline implementation
type FunctionalPipeline[T, R any] struct {
	process ProcessFunc[T, R]
	name    string
	config  *Config
}

// NewFunctional creates a functional Pipeline
func NewFunctional[T, R any](name string, process ProcessFunc[T, R]) *FunctionalPipeline[T, R] {
	// Validate process function
	if process == nil {
		panic("process function cannot be nil")
	}
	
	return &FunctionalPipeline[T, R]{
		process: process,
		name:    name,
		config: &Config{
			Workers:      1,
			BufferSize:   100,
			Timeout:      30 * time.Second,
			StageTimeout: 10 * time.Second,
			MaxRetries:   3,
			RetryDelay:   time.Second,
			Clock:        types.NewRealClock(),
		},
	}
}

// Execute executes the processing
func (p *FunctionalPipeline[T, R]) Execute(ctx context.Context, input T) (R, error) {
	// Add timeout control
	if p.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.config.Timeout)
		defer cancel()
	}

	return p.process(ctx, input)
}

// WithTimeout sets timeout
func (p *FunctionalPipeline[T, R]) WithTimeout(timeout time.Duration) *FunctionalPipeline[T, R] {
	newConfig := *p.config
	newConfig.Timeout = timeout
	// Ensure clock is preserved
	if newConfig.Clock == nil {
		newConfig.Clock = types.NewRealClock()
	}
	return &FunctionalPipeline[T, R]{
		process: p.process,
		name:    p.name,
		config:  &newConfig,
	}
}

// GetClock returns the pipeline's clock
func (p *FunctionalPipeline[T, R]) GetClock() types.Clock {
	return p.config.Clock
}

// NOTE: Chain, Chain3, Chain4 have been replaced by more flexible scalable chaining API
// Please use DynamicChain, ArrayChain, ImmutableChain or ChainBuilder

// WithRetry adds retry functionality
func WithRetry[T, R any](fn ProcessFunc[T, R], maxRetries int, delay time.Duration) ProcessFunc[T, R] {
	return func(ctx context.Context, input T) (R, error) {
		var lastErr error
		var zero R

		// Get clock from context, fallback to real clock
		clock := types.ClockFromContext(ctx)

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				select {
				case <-clock.After(delay):
				case <-ctx.Done():
					return zero, ctx.Err()
				}
			}

			result, err := fn(ctx, input)
			if err == nil {
				return result, nil
			}

			lastErr = err

			// Check if retryable
			if retryableErr, ok := err.(interface{ IsRetryable() bool }); ok {
				if !retryableErr.IsRetryable() {
					break
				}
			}
		}

		return zero, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
	}
}

// HandleErrors adds error handling
func HandleErrors[T, R any](fn ProcessFunc[T, R], handler func(error) error) ProcessFunc[T, R] {
	return func(ctx context.Context, input T) (R, error) {
		result, err := fn(ctx, input)
		if err != nil {
			err = handler(err)
		}
		return result, err
	}
}

// Transform function mapping - simplest transformation
func Transform[T, R any](transform func(T) R) ProcessFunc[T, R] {
	return func(ctx context.Context, input T) (R, error) {
		return transform(input), nil
	}
}

// TryTransform potentially failing mapping
func TryTransform[T, R any](transform func(T) (R, error)) ProcessFunc[T, R] {
	return func(ctx context.Context, input T) (R, error) {
		return transform(input)
	}
}

// FilterFunc filter - returns value and whether it passes the filter
func FilterFunc[T any](predicate func(T) bool) ProcessFunc[T, T] {
	return func(ctx context.Context, input T) (T, error) {
		if predicate(input) {
			return input, nil
		}
		var zero T
		return zero, fmt.Errorf("filtered out")
	}
}

// FilterWithResult filter - returns value, whether it passes and error
func FilterWithResult[T any](predicate func(T) (bool, error)) ProcessFunc[T, T] {
	return func(ctx context.Context, input T) (T, error) {
		pass, err := predicate(input)
		if err != nil {
			var zero T
			return zero, err
		}
		if pass {
			return input, nil
		}
		var zero T
		return zero, fmt.Errorf("filtered out")
	}
}

// Conditional conditional execution
func Conditional[T, R any](
	condition func(T) bool,
	trueBranch ProcessFunc[T, R],
	falseBranch ProcessFunc[T, R],
) ProcessFunc[T, R] {
	return func(ctx context.Context, input T) (R, error) {
		if condition(input) {
			return trueBranch(ctx, input)
		}
		return falseBranch(ctx, input)
	}
}

// Parallel executes in parallel (returns two results)
// Optimized version: uses sync.WaitGroup and shared struct, avoids creating channels
func Parallel[T, R1, R2 any](
	fn1 ProcessFunc[T, R1],
	fn2 ProcessFunc[T, R2],
) ProcessFunc[T, struct {
	R1 R1
	R2 R2
}] {
	return func(ctx context.Context, input T) (struct {
		R1 R1
		R2 R2
	}, error) {
		// Use struct and mutex to store results, avoid creating channels
		var result struct {
			r1   R1
			r2   R2
			err1 error
			err2 error
			mu   sync.Mutex
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Execute first function
		go func() {
			defer wg.Done()
			val, err := fn1(ctx, input)
			result.mu.Lock()
			result.r1 = val
			result.err1 = err
			result.mu.Unlock()
		}()

		// Execute second function
		go func() {
			defer wg.Done()
			val, err := fn2(ctx, input)
			result.mu.Lock()
			result.r2 = val
			result.err2 = err
			result.mu.Unlock()
		}()

		// Wait for completion in a separate goroutine
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		var zero struct {
			R1 R1
			R2 R2
		}

		// Wait for completion or context cancellation
		select {
		case <-done:
			// Both goroutines completed
		case <-ctx.Done():
			return zero, ctx.Err()
		}

		// Check errors
		result.mu.Lock()
		err1, err2 := result.err1, result.err2
		r1, r2 := result.r1, result.r2
		result.mu.Unlock()

		if err1 != nil {
			return zero, fmt.Errorf("first branch failed: %w", err1)
		}
		if err2 != nil {
			return zero, fmt.Errorf("second branch failed: %w", err2)
		}

		return struct {
			R1 R1
			R2 R2
		}{R1: r1, R2: r2}, nil
	}
}

// Tap observe intermediate values (for debugging)
func Tap[T any](fn ProcessFunc[T, T], observer func(T)) ProcessFunc[T, T] {
	return func(ctx context.Context, input T) (T, error) {
		result, err := fn(ctx, input)
		if err == nil {
			observer(result)
		}
		return result, err
	}
}
