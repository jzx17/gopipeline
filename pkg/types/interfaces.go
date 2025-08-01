// Package types defines core interfaces and types for the Pipeline library
package types

import (
	"context"
	"time"
)

// PipelineExecutor defines the execution interface for pipelines - focused on data processing
type PipelineExecutor[T, R any] interface {
	// Execute executes pipeline processing
	Execute(ctx context.Context, input T) (R, error)

	// ExecuteAsync executes pipeline processing asynchronously
	ExecuteAsync(ctx context.Context, input T) <-chan Result[R]

	// ExecuteBatch executes pipeline processing in batch
	ExecuteBatch(ctx context.Context, inputs []T) <-chan BatchResult[R]
}

// PipelineManager defines the lifecycle management interface for pipelines
type PipelineManager interface {
	// Start starts the pipeline
	Start() error

	// Stop stops the pipeline
	Stop() error

	// Close closes the pipeline and releases resources
	Close() error

	// GetState returns the pipeline state
	GetState() PipelineState
}

// ClockProvider provides access to clock for testing
type ClockProvider interface {
	GetClock() Clock
}

// Pipeline interface - focused on execution and lifecycle management
// Use functional API for pipeline composition, see functional.go and scalable_chain.go
type Pipeline[T, R any] interface {
	PipelineExecutor[T, R]
	PipelineManager
	ClockProvider
}

// PipelineState defines the state of a Pipeline
type PipelineState int

const (
	// StateCreated Pipeline has been created but not started
	StateCreated PipelineState = iota
	// StateRunning Pipeline is running
	StateRunning
	// StateStopped Pipeline has been stopped
	StateStopped
	// StateClosed Pipeline has been closed
	StateClosed
)

// String returns the string representation of PipelineState
func (ps PipelineState) String() string {
	switch ps {
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

// Stream defines the streaming processing interface
type Stream[T any] interface {
	// Send sends data to the stream
	Send(ctx context.Context, data T) error

	// Receive receives data from the stream
	Receive(ctx context.Context) (T, error)

	// Close closes the stream
	Close() error
}

// WorkerPool defines the worker pool interface
type WorkerPool interface {
	// Submit submits a task to the worker pool
	Submit(task Task) error

	// SubmitWithTimeout submits a task to the worker pool with timeout
	SubmitWithTimeout(task Task, timeout time.Duration) error

	// Start starts the worker pool
	Start(ctx context.Context) error

	// Stop stops the worker pool
	Stop() error

	// Close closes the worker pool and releases resources
	Close() error

	// Size returns the size of the worker pool
	Size() int

	// Stats returns worker pool statistics
	Stats() WorkerPoolStats
}

// DynamicWorkerPool defines the extended interface for dynamic worker pools
type DynamicWorkerPool interface {
	WorkerPool

	// GetMinWorkers returns the minimum number of workers
	GetMinWorkers() int

	// GetMaxWorkers returns the maximum number of workers
	GetMaxWorkers() int

	// GetCurrentWorkers returns the current number of workers
	GetCurrentWorkers() int

	// ScaleUp scales up to the specified number of workers
	ScaleUp(targetSize int) error

	// ScaleDown scales down to the specified number of workers
	ScaleDown(targetSize int) error
}

// PriorityWorkerPool defines the extended interface for priority worker pools
type PriorityWorkerPool interface {
	WorkerPool

	// SubmitWithPriority submits a task with specified priority
	SubmitWithPriority(task Task, priority int) error

	// SubmitWithPriorityAndTimeout submits a task with priority and timeout
	SubmitWithPriorityAndTimeout(task Task, priority int, timeout time.Duration) error

	// UpdatePriority dynamically updates task priority
	UpdatePriority(taskID string, priority int) error

	// GetPriorityStats returns priority statistics
	GetPriorityStats() PriorityStats
}

// Task defines the task interface
type Task interface {
	// Execute executes the task
	Execute(ctx context.Context) error

	// ID returns the task ID (optional, for tracking)
	ID() string

	// Priority returns the task priority (optional, for sorting)
	Priority() int
}

// WorkerPoolStats defines basic statistics for worker pools
type WorkerPoolStats struct {
	// PoolSize is the size of the pool
	PoolSize int

	// ActiveWorkers is the number of active worker goroutines
	ActiveWorkers int

	// QueueSize is the current number of tasks in the queue
	QueueSize int

	// QueueCapacity is the capacity of the queue
	QueueCapacity int
}

// ErrorHandler defines an error handling function (simple version for backward compatibility)
// For advanced error handling, use the ErrorHandler interface in the internal/errors package
type ErrorHandler func(error) error

// RetryPolicy defines the retry policy interface
type RetryPolicy interface {
	// ShouldRetry determines whether a retry should be attempted
	ShouldRetry(err error, attempt int) bool

	// NextDelay returns the delay before the next retry
	NextDelay(attempt int) time.Duration
}

// Option defines a configuration option function
type Option[T any] func(T)

// Config defines Pipeline configuration
type Config struct {
	// Workers is the number of worker goroutines
	Workers int

	// BufferSize is the size of buffers
	BufferSize int

	// Timeout is the global timeout duration
	Timeout time.Duration

	// StageTimeout is the timeout duration for individual stages
	StageTimeout time.Duration

	// MaxRetries is the maximum number of retries
	MaxRetries int

	// RetryDelay is the delay between retries
	RetryDelay time.Duration

	// EnableTracing indicates whether tracing is enabled
	EnableTracing bool

	// Clock provides time operations (for testing)
	Clock Clock
}

// Result defines the result of asynchronous execution
type Result[R any] struct {
	// Value is the execution result
	Value R

	// Error is the execution error
	Error error

	// Duration is the execution time
	Duration time.Duration
}

// BatchResult defines the result of batch execution
type BatchResult[R any] struct {
	// Index is the index of the input data
	Index int

	// Value is the execution result
	Value R

	// Error is the execution error
	Error error

	// Duration is the execution time
	Duration time.Duration
}

// PriorityStats defines statistics for priority queues
type PriorityStats struct {
	// QueueLengthByPriority shows queue lengths by priority level
	QueueLengthByPriority map[int]int

	// TasksByPriority shows total tasks by priority level
	TasksByPriority map[int]int64

	// CompletedByPriority shows completed tasks by priority level
	CompletedByPriority map[int]int64

	// Task waiting time statistics
	AverageWaitTime   time.Duration
	MaxWaitTime       time.Duration
	TotalWaitingTasks int64

	// Starvation detection metrics
	StarvationDetected int64 // Number of tasks detected as starved
	AgePromotions      int64 // Total number of age-based promotions
}
