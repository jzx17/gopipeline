package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBasicTask(t *testing.T) {
	called := false
	fn := func(ctx context.Context) error {
		called = true
		return nil
	}

	task := NewBasicTask(fn)

	assert.NotEmpty(t, task.ID())
	assert.Equal(t, 0, task.Priority())

	// Execute task
	err := task.Execute(context.Background())
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestNewBasicTaskWithPriority(t *testing.T) {
	fn := func(ctx context.Context) error {
		return nil
	}

	task := NewBasicTaskWithPriority(fn, 5)

	assert.NotEmpty(t, task.ID())
	assert.Equal(t, 5, task.Priority())
}

func TestNewBasicTaskWithID(t *testing.T) {
	fn := func(ctx context.Context) error {
		return nil
	}

	customID := "custom-task-123"
	task := NewBasicTaskWithID(customID, fn)

	assert.Equal(t, customID, task.ID())
	assert.Equal(t, 0, task.Priority())
}

func TestBasicTask_Execute(t *testing.T) {
	tests := []struct {
		name        string
		fn          func(ctx context.Context) error
		expectError bool
	}{
		{
			name: "successful execution",
			fn: func(ctx context.Context) error {
				return nil
			},
			expectError: false,
		},
		{
			name: "failed execution",
			fn: func(ctx context.Context) error {
				return fmt.Errorf("task failed")
			},
			expectError: true,
		},
		{
			name:        "nil function",
			fn:          nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewBasicTask(tt.fn)
			err := task.Execute(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBasicTask_SetPriority(t *testing.T) {
	task := NewBasicTask(func(ctx context.Context) error {
		return nil
	})

	assert.Equal(t, 0, task.Priority())

	task.SetPriority(10)
	assert.Equal(t, 10, task.Priority())

	task.SetPriority(-5)
	assert.Equal(t, -5, task.Priority())
}

func TestTaskWithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		taskDelay   time.Duration
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "task completes before timeout",
			taskDelay:   10 * time.Millisecond,
			timeout:     50 * time.Millisecond,
			expectError: false,
		},
		{
			name:        "task times out",
			taskDelay:   100 * time.Millisecond,
			timeout:     20 * time.Millisecond,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalTask := NewBasicTask(func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(tt.taskDelay):
					return nil
				}
			})

			timeoutCtx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			timeoutTask := NewTaskWithTimeout(originalTask, timeoutCtx, cancel)

			err := timeoutTask.Execute(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Cleanup
			timeoutTask.Cleanup()
		})
	}
}

func TestTaskWithTimeout_ContextMerging(t *testing.T) {
	// Create a task that checks context cancellation
	originalTask := NewBasicTask(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})

	// Create timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer timeoutCancel()

	timeoutTask := NewTaskWithTimeout(originalTask, timeoutCtx, timeoutCancel)

	// Create another cancellable context
	parentCtx, parentCancel := context.WithCancel(context.Background())

	// Cancel parent context after some time
	go func() {
		time.Sleep(30 * time.Millisecond)
		parentCancel()
	}()

	// Execute task, should end early due to parent context cancellation
	err := timeoutTask.Execute(parentCtx)
	assert.Error(t, err)

	timeoutTask.Cleanup()
}

func TestTaskIDCounter(t *testing.T) {
	// Create multiple tasks, verify IDs are unique
	tasks := make([]*BasicTask, 10)
	ids := make(map[string]bool)

	for i := 0; i < 10; i++ {
		tasks[i] = NewBasicTask(func(ctx context.Context) error {
			return nil
		})

		id := tasks[i].ID()
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "ID should be unique: %s", id)
		ids[id] = true
	}
}

func TestTaskWithTimeout_Cleanup(t *testing.T) {
	originalTask := NewBasicTask(func(ctx context.Context) error {
		return nil
	})

	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)

	timeoutTask := NewTaskWithTimeout(originalTask, timeoutCtx, cancel)

	// Execute task
	err := timeoutTask.Execute(context.Background())
	assert.NoError(t, err)

	// Cleanup should call cancel function
	timeoutTask.Cleanup()

	// Verify context has been cancelled
	assert.Error(t, timeoutCtx.Err())
}

// Benchmark tests
func BenchmarkBasicTask_Execute(b *testing.B) {
	task := NewBasicTask(func(ctx context.Context) error {
		return nil
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = task.Execute(ctx)
	}
}

func BenchmarkNewBasicTask(b *testing.B) {
	fn := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBasicTask(fn)
	}
}

func BenchmarkPriorityTask_Execute(b *testing.B) {
	originalTask := NewBasicTask(func(ctx context.Context) error {
		return nil
	})

	priorityTask := NewPriorityTask(originalTask, 5)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = priorityTask.Execute(ctx)
	}
}
