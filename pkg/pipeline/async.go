// Package pipeline provides simplified asynchronous Pipeline implementation
package pipeline

import (
	"context"
	"sync"

	"github.com/jzx17/gopipeline/pkg/types"
)

// AsyncOptions simplified asynchronous execution options
type AsyncOptions struct {
	// BufferSize result buffer size
	BufferSize int
	// Concurrency level of concurrent execution
	Concurrency int
}

// DefaultAsyncOptions returns default asynchronous options
func DefaultAsyncOptions() *AsyncOptions {
	return &AsyncOptions{
		BufferSize:  100,
		Concurrency: 10,
	}
}

// AsyncPipeline simplified asynchronous Pipeline wrapper
type AsyncPipeline[T, R any] struct {
	pipeline types.Pipeline[T, R]
	options  *AsyncOptions
	clock    types.Clock
}

// NewAsyncPipeline creates a new asynchronous Pipeline
func NewAsyncPipeline[T, R any](pipeline types.Pipeline[T, R], opts ...*AsyncOptions) *AsyncPipeline[T, R] {
	options := DefaultAsyncOptions()
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	// Extract clock from pipeline
	clock := pipeline.GetClock()

	return &AsyncPipeline[T, R]{
		pipeline: pipeline,
		options:  options,
		clock:    clock,
	}
}

// ExecuteAsync executes a single task asynchronously
func (ap *AsyncPipeline[T, R]) ExecuteAsync(ctx context.Context, input T) <-chan types.Result[R] {
	resultChan := make(chan types.Result[R], 1)

	go func() {
		defer close(resultChan)

		start := ap.clock.Now()
		result, err := ap.pipeline.Execute(ctx, input)
		duration := ap.clock.Since(start)

		// Use object pool to reduce allocations
		pooledResult := types.GetPooledResult[R]()
		pooledResult.Value = result
		pooledResult.Error = err
		pooledResult.Duration = duration

		resultChan <- *pooledResult
		// Return to pool after sending (receiver gets a copy)
		types.PutPooledResult(pooledResult)
	}()

	return resultChan
}

// ExecuteBatch executes batch tasks asynchronously
func (ap *AsyncPipeline[T, R]) ExecuteBatch(ctx context.Context, inputs []T) <-chan types.BatchResult[R] {
	resultChan := make(chan types.BatchResult[R], len(inputs))

	go func() {
		defer close(resultChan)

		var wg sync.WaitGroup
		semaphore := make(chan struct{}, ap.options.Concurrency)

		for i, input := range inputs {
			wg.Add(1)
			go func(index int, inp T) {
				defer wg.Done()

				// Control concurrency level
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				start := ap.clock.Now()
				result, err := ap.pipeline.Execute(ctx, inp)
				duration := ap.clock.Since(start)

				// Use object pool to reduce allocations
				pooledResult := types.GetPooledBatchResult[R]()
				pooledResult.Index = index
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
			}(i, input)
		}

		wg.Wait()
	}()

	return resultChan
}

// Close closes the asynchronous Pipeline
func (ap *AsyncPipeline[T, R]) Close() error {
	if ap.pipeline != nil {
		return ap.pipeline.Close()
	}
	return nil
}
