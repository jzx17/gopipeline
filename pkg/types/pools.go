// Package types provides object pools for performance optimization
package types

import (
	"sync"
)

// ResultPool manages Result[T] object pooling to reduce GC pressure
type ResultPool[T any] struct {
	pool sync.Pool
}

// NewResultPool creates a new result pool for type T
func NewResultPool[T any]() *ResultPool[T] {
	return &ResultPool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return &Result[T]{}
			},
		},
	}
}

// Get retrieves a Result[T] from the pool or creates a new one
func (rp *ResultPool[T]) Get() *Result[T] {
	return rp.pool.Get().(*Result[T])
}

// Put returns a Result[T] to the pool after resetting it
func (rp *ResultPool[T]) Put(result *Result[T]) {
	if result != nil {
		// Reset the result to prevent memory leaks
		var zero T
		result.Value = zero
		result.Error = nil
		result.Duration = 0
		rp.pool.Put(result)
	}
}

// BatchResultPool manages BatchResult[T] object pooling
type BatchResultPool[T any] struct {
	pool sync.Pool
}

// NewBatchResultPool creates a new batch result pool for type T
func NewBatchResultPool[T any]() *BatchResultPool[T] {
	return &BatchResultPool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return &BatchResult[T]{}
			},
		},
	}
}

// Get retrieves a BatchResult[T] from the pool or creates a new one
func (brp *BatchResultPool[T]) Get() *BatchResult[T] {
	return brp.pool.Get().(*BatchResult[T])
}

// Put returns a BatchResult[T] to the pool after resetting it
func (brp *BatchResultPool[T]) Put(result *BatchResult[T]) {
	if result != nil {
		// Reset the batch result to prevent memory leaks
		var zero T
		result.Index = 0
		result.Value = zero
		result.Error = nil
		result.Duration = 0
		brp.pool.Put(result)
	}
}

// ErrorPool manages error object pooling for common error types
type ErrorPool struct {
	pipelineErrorPool sync.Pool
}

// NewErrorPool creates a new error pool
func NewErrorPool() *ErrorPool {
	return &ErrorPool{
		pipelineErrorPool: sync.Pool{
			New: func() interface{} {
				return &PipelineError{}
			},
		},
	}
}

// GetPipelineError retrieves a PipelineError from the pool
func (ep *ErrorPool) GetPipelineError() *PipelineError {
	return ep.pipelineErrorPool.Get().(*PipelineError)
}

// PutPipelineError returns a PipelineError to the pool after resetting it
func (ep *ErrorPool) PutPipelineError(err *PipelineError) {
	if err != nil {
		// Reset the error to prevent memory leaks
		err.Operation = ""
		err.Input = nil
		err.Cause = nil
		err.Context = nil
		ep.pipelineErrorPool.Put(err)
	}
}

// Global pools for common usage
var (
	// GlobalErrorPool provides a global error pool instance
	GlobalErrorPool = NewErrorPool()
)

// Helper functions for easy pool usage

// GetPooledResult gets a result from a global pool (convenience function)
func GetPooledResult[T any]() *Result[T] {
	// Create a type-specific pool on demand
	// Note: In practice, you'd want to cache these pools by type
	pool := NewResultPool[T]()
	return pool.Get()
}

// PutPooledResult returns a result to the global pool
func PutPooledResult[T any](result *Result[T]) {
	if result != nil {
		pool := NewResultPool[T]()
		pool.Put(result)
	}
}

// GetPooledBatchResult gets a batch result from a global pool
func GetPooledBatchResult[T any]() *BatchResult[T] {
	pool := NewBatchResultPool[T]()
	return pool.Get()
}

// PutPooledBatchResult returns a batch result to the global pool
func PutPooledBatchResult[T any](result *BatchResult[T]) {
	if result != nil {
		pool := NewBatchResultPool[T]()
		pool.Put(result)
	}
}