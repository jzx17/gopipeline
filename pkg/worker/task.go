// Package worker provides worker pool implementations
package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// taskIDCounter is the global task ID counter
var taskIDCounter int64

// BasicTask is the basic implementation of Task interface
type BasicTask struct {
	id       string
	priority int
	fn       func(ctx context.Context) error
}

// NewBasicTask creates a new basic task
func NewBasicTask(fn func(ctx context.Context) error) *BasicTask {
	id := atomic.AddInt64(&taskIDCounter, 1)
	return &BasicTask{
		id:       fmt.Sprintf("task-%d", id),
		priority: 0,
		fn:       fn,
	}
}

// NewBasicTaskWithPriority creates a basic task with priority
func NewBasicTaskWithPriority(fn func(ctx context.Context) error, priority int) *BasicTask {
	id := atomic.AddInt64(&taskIDCounter, 1)
	return &BasicTask{
		id:       fmt.Sprintf("task-%d", id),
		priority: priority,
		fn:       fn,
	}
}

// NewBasicTaskWithID creates a basic task with custom ID
func NewBasicTaskWithID(id string, fn func(ctx context.Context) error) *BasicTask {
	return &BasicTask{
		id:       id,
		priority: 0,
		fn:       fn,
	}
}

// Execute executes the task
func (t *BasicTask) Execute(ctx context.Context) error {
	if t.fn == nil {
		return fmt.Errorf("task %s has no execution function", t.id)
	}
	return t.fn(ctx)
}

// ID returns the task ID
func (t *BasicTask) ID() string {
	return t.id
}

// Priority returns the task priority
func (t *BasicTask) Priority() int {
	return t.priority
}

// SetPriority sets the task priority
func (t *BasicTask) SetPriority(priority int) {
	t.priority = priority
}

// TaskWithTimeout is a task wrapper with timeout
type TaskWithTimeout struct {
	types.Task
	timeout context.Context
	cancel  context.CancelFunc
	clock   types.Clock
}

// NewTaskWithTimeout creates a task with timeout
func NewTaskWithTimeout(task types.Task, timeout context.Context, cancel context.CancelFunc) *TaskWithTimeout {
	return NewTaskWithTimeoutAndClock(task, timeout, cancel, types.NewRealClock())
}

// NewTaskWithTimeoutAndClock creates a task with timeout and custom clock
func NewTaskWithTimeoutAndClock(task types.Task, timeout context.Context, cancel context.CancelFunc, clock types.Clock) *TaskWithTimeout {
	if clock == nil {
		clock = types.NewRealClock()
	}
	return &TaskWithTimeout{
		Task:    task,
		timeout: timeout,
		cancel:  cancel,
		clock:   clock,
	}
}

// Execute executes the task with timeout
func (tt *TaskWithTimeout) Execute(ctx context.Context) error {
	// Use stricter timeout context
	mergedCtx := tt.timeout
	if ctx.Done() != nil {
		var mergedCancel context.CancelFunc
		mergedCtx, mergedCancel = context.WithCancel(tt.timeout)

		// Create goroutine with cleanup mechanism
		done := make(chan struct{})
		go func() {
			defer close(done)
			select {
			case <-ctx.Done():
				mergedCancel()
			case <-tt.timeout.Done():
				mergedCancel()
			}
		}()

		// Ensure goroutine is properly cleaned up
		defer func() {
			mergedCancel()
			// Wait for goroutine to exit with reasonable timeout
			select {
			case <-done:
				// Goroutine exited normally
			case <-tt.clock.After(100 * time.Millisecond):
				// Timeout, but goroutine should exit due to context cancellation
			}
		}()
	}

	return tt.Task.Execute(mergedCtx)
}

// Cleanup cleans up task resources
func (tt *TaskWithTimeout) Cleanup() {
	if tt.cancel != nil {
		tt.cancel()
	}
}
