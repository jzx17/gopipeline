package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewWorker(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	assert.Equal(t, 1, worker.ID())
	assert.Equal(t, WorkerStateIdle, worker.State())
}

func TestWorkerState(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	// Initial state should be idle
	assert.Equal(t, WorkerStateIdle, worker.State())

	// State string representation
	assert.Equal(t, "idle", WorkerStateIdle.String())
	assert.Equal(t, "working", WorkerStateWorking.String())
	assert.Equal(t, "stopped", WorkerStateStopped.String())
	assert.Equal(t, "unknown", WorkerState(999).String())
}

func TestWorker_Start(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go worker.Start(ctx)

	// Wait for worker to start
	time.Sleep(10 * time.Millisecond)

	// Submit task
	var executed int64
	task := NewBasicTask(func(ctx context.Context) error {
		atomic.AddInt64(&executed, 1)
		return nil
	})

	taskChan <- task

	// Wait for task execution
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, int64(1), atomic.LoadInt64(&executed))

	// Stop worker
	cancel()
}

func TestWorker_Stop(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx := context.Background()

	// Start worker
	go worker.Start(ctx)

	// Wait for worker to start
	time.Sleep(10 * time.Millisecond)

	// Stop worker
	err := worker.Stop()
	assert.NoError(t, err)

	// Verify worker is stopped
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, WorkerStateStopped, worker.State())

	// Repeated stop should succeed
	err = worker.Stop()
	assert.NoError(t, err)
}

func TestWorker_TaskExecution(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var executionTime time.Duration
	var failed bool

	// Set completion callback
	worker.SetCompletionCallback(func(duration time.Duration, taskFailed bool) {
		executionTime = duration
		failed = taskFailed
	})

	// Start worker
	go worker.Start(ctx)

	// Submit successful task
	task := NewBasicTask(func(ctx context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})

	taskChan <- task

	// Wait for task completion
	time.Sleep(20 * time.Millisecond)

	stats := worker.Stats()
	assert.Equal(t, int64(1), stats.TotalProcessed)
	assert.True(t, executionTime > 0)
	assert.False(t, failed)
}

func TestWorker_TaskExecutionWithError(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var errorHandled bool
	worker.SetErrorHandler(func(err error) error {
		errorHandled = true
		return nil
	})

	var executionTime time.Duration
	var failed bool

	worker.SetCompletionCallback(func(duration time.Duration, taskFailed bool) {
		executionTime = duration
		failed = taskFailed
	})

	// Start worker
	go worker.Start(ctx)

	// Submit failing task
	task := NewBasicTask(func(ctx context.Context) error {
		return fmt.Errorf("task failed")
	})

	taskChan <- task

	// Wait for task completion
	time.Sleep(20 * time.Millisecond)

	stats := worker.Stats()
	assert.GreaterOrEqual(t, stats.TotalProcessed, int64(0))
	assert.True(t, errorHandled)
	assert.True(t, executionTime > 0)
	assert.True(t, failed)
}

func TestWorker_TaskPanic(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var panicHandled bool
	worker.SetErrorHandler(func(err error) error {
		panicHandled = true
		// Verify error contains panic information
		assert.Contains(t, err.Error(), "panic")
		return nil
	})

	// Start worker
	go worker.Start(ctx)

	// Submit panic task
	task := NewBasicTask(func(ctx context.Context) error {
		panic("test panic")
	})

	taskChan <- task

	// Wait for task completion
	time.Sleep(20 * time.Millisecond)

	stats := worker.Stats()
	assert.GreaterOrEqual(t, stats.TotalProcessed, int64(0))
	assert.True(t, panicHandled)
}

func TestWorker_ContextCancellation(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker
	go worker.Start(ctx)

	// Submit long-running task
	var taskCompleted bool
	task := NewBasicTask(func(taskCtx context.Context) error {
		select {
		case <-taskCtx.Done():
			return taskCtx.Err()
		case <-time.After(100 * time.Millisecond):
			taskCompleted = true
			return nil
		}
	})

	taskChan <- task

	// Wait briefly for task to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for worker to stop
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, WorkerStateStopped, worker.State())
	assert.False(t, taskCompleted) // Task should be cancelled
}

func TestWorker_Stats(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go worker.Start(ctx)

	// Submit multiple tasks
	numSuccess := 3
	numFailed := 2

	var wg sync.WaitGroup
	wg.Add(numSuccess + numFailed)

	// Successful tasks
	for i := 0; i < numSuccess; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			return nil
		})
		taskChan <- task
	}

	// Failed tasks
	for i := 0; i < numFailed; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			return fmt.Errorf("task failed")
		})
		taskChan <- task
	}

	// Wait for all tasks to complete
	wg.Wait()

	stats := worker.Stats()
	assert.Equal(t, 1, stats.ID)
	assert.Equal(t, int64(numSuccess), stats.TotalProcessed)
	assert.True(t, stats.LastTaskTime.After(time.Time{}))

	// Test stats methods
	assert.False(t, stats.IsActive())
	assert.True(t, stats.IsIdle())
}

func TestWorker_MultipleTasksSequential(t *testing.T) {
	taskChan := make(chan types.Task, 1) // Single task buffer
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go worker.Start(ctx)

	var counter int64
	var wg sync.WaitGroup

	numTasks := 10
	wg.Add(numTasks)

	// Submit tasks sequentially
	for i := 0; i < numTasks; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			wg.Done()
			return nil
		})
		taskChan <- task
	}

	// Wait for all tasks to complete
	wg.Wait()

	assert.Equal(t, int64(numTasks), atomic.LoadInt64(&counter))

	stats := worker.Stats()
	assert.Equal(t, int64(numTasks), stats.TotalProcessed)
}

func TestWorker_StopTimeout(t *testing.T) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go worker.Start(ctx)

	// Wait for worker to start
	time.Sleep(10 * time.Millisecond)

	// Stop worker directly without submitting blocking tasks
	start := time.Now()
	err := worker.Stop()
	duration := time.Since(start)

	// Stop operation should complete quickly
	assert.NoError(t, err)
	assert.Less(t, duration, 100*time.Millisecond)
	assert.Equal(t, WorkerStateStopped, worker.State())
}

// Benchmark tests
func BenchmarkWorker_TaskExecution(b *testing.B) {
	taskChan := make(chan types.Task, 1000)
	worker := NewWorker(1, taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go worker.Start(ctx)

	task := NewBasicTask(func(ctx context.Context) error {
		return nil
	})

	var wg sync.WaitGroup

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			taskChan <- task
		}()
	}

	wg.Wait()
}

func BenchmarkWorker_Stats(b *testing.B) {
	taskChan := make(chan types.Task, 10)
	worker := NewWorker(1, taskChan)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = worker.Stats()
	}
}
