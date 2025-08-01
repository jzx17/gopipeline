// Package types defines error types
package types

import (
	"errors"
	"fmt"
	"time"
)

// Predefined errors
var (
	// ErrPipelineClosed indicates the pipeline is closed
	ErrPipelineClosed = errors.New("pipeline is closed")

	// ErrPipelineStopped indicates the pipeline is stopped
	ErrPipelineStopped = errors.New("pipeline is stopped")

	// ErrInvalidInput indicates invalid input
	ErrInvalidInput = errors.New("invalid input")

	// ErrTimeout indicates operation timeout
	ErrTimeout = errors.New("operation timeout")

	// ErrFiltered indicates data was filtered
	ErrFiltered = errors.New("data filtered")

	// ErrWorkerPoolFull indicates the worker pool is full
	ErrWorkerPoolFull = errors.New("worker pool is full")

	// ErrStreamClosed indicates the stream is closed
	ErrStreamClosed = errors.New("stream is closed")
)

// TypedPipelineError represents a type-safe Pipeline processing error
type TypedPipelineError[T any] struct {
	// Operation is the name of the operation where the error occurred
	Operation string

	// Input is the input data that caused the error (type-safe)
	Input T

	// Cause is the underlying error
	Cause error

	// Context contains error context information
	Context map[string]interface{}
}

// Error implements the error interface
func (e *TypedPipelineError[T]) Error() string {
	return fmt.Sprintf("pipeline error in operation %s: %v", e.Operation, e.Cause)
}

// Unwrap returns the underlying error
func (e *TypedPipelineError[T]) Unwrap() error {
	return e.Cause
}

// Is checks if the error is a specific error
func (e *TypedPipelineError[T]) Is(target error) bool {
	return errors.Is(e.Cause, target)
}

// NewTypedPipelineError creates a new type-safe Pipeline error
func NewTypedPipelineError[T any](operation string, input T, cause error) *TypedPipelineError[T] {
	return &TypedPipelineError[T]{
		Operation: operation,
		Input:     input,
		Cause:     cause,
		Context:   make(map[string]interface{}),
	}
}

// WithContext adds error context
func (e *TypedPipelineError[T]) WithContext(key string, value interface{}) *TypedPipelineError[T] {
	e.Context[key] = value
	return e
}

// PipelineError represents an error in Pipeline processing (deprecated: use TypedPipelineError)
// Deprecated: Use TypedPipelineError for type safety
type PipelineError struct {
	// Operation is the name of the operation where the error occurred
	Operation string

	// Input is the input data that caused the error
	Input interface{}

	// Cause is the underlying error
	Cause error

	// Context contains error context information
	Context map[string]interface{}
}

// Error implements the error interface
func (e *PipelineError) Error() string {
	return fmt.Sprintf("pipeline error in operation %s: %v", e.Operation, e.Cause)
}

// Unwrap returns the underlying error
func (e *PipelineError) Unwrap() error {
	return e.Cause
}

// Is checks if the error is a specific error
func (e *PipelineError) Is(target error) bool {
	return errors.Is(e.Cause, target)
}

// NewPipelineError creates a new Pipeline error (deprecated: use NewTypedPipelineError)
// Deprecated: Use NewTypedPipelineError for type safety
func NewPipelineError(operation string, input interface{}, cause error) *PipelineError {
	return &PipelineError{
		Operation: operation,
		Input:     input,
		Cause:     cause,
		Context:   make(map[string]interface{}),
	}
}

// WithContext adds error context
func (e *PipelineError) WithContext(key string, value interface{}) *PipelineError {
	e.Context[key] = value
	return e
}

// RetryableError represents a retryable error
type RetryableError struct {
	// Err is the underlying error
	Err error

	// Retryable indicates whether the error is retryable
	Retryable bool

	// RetryAfter is the suggested retry delay
	RetryAfter time.Duration
}

// Error implements the error interface
func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.Retryable
	}
	return false
}

// GetRetryDelay returns the suggested retry delay
func GetRetryDelay(err error) time.Duration {
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.RetryAfter
	}
	return 0
}
