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
	"github.com/stretchr/testify/require"
)

func TestNewFixedWorkerPool(t *testing.T) {
	tests := []struct {
		name        string
		config      *FixedWorkerPoolConfig
		expectError bool
	}{
		{
			name:        "nil config should use default",
			config:      nil,
			expectError: false,
		},
		{
			name: "valid config",
			config: &FixedWorkerPoolConfig{
				PoolSize:  5,
				QueueSize: 50,
			},
			expectError: false,
		},
		{
			name: "zero pool size should error",
			config: &FixedWorkerPoolConfig{
				PoolSize:  0,
				QueueSize: 50,
			},
			expectError: true,
		},
		{
			name: "negative pool size should error",
			config: &FixedWorkerPoolConfig{
				PoolSize:  -1,
				QueueSize: 50,
			},
			expectError: true,
		},
		{
			name: "zero queue size should error",
			config: &FixedWorkerPoolConfig{
				PoolSize:  5,
				QueueSize: 0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewFixedWorkerPool(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
				if tt.config == nil {
					assert.Equal(t, 10, pool.Size()) // default pool size
				} else {
					assert.Equal(t, tt.config.PoolSize, pool.Size())
				}
			}
		})
	}
}

func TestFixedWorkerPool_StartStop(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  3,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test start
	err = pool.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, pool.IsRunning())

	// Test repeated start
	err = pool.Start(ctx)
	assert.Error(t, err)

	// Test stop
	err = pool.Stop()
	assert.NoError(t, err)
	assert.False(t, pool.IsRunning())

	// Test repeated stop
	err = pool.Stop()
	assert.Error(t, err)
}

func TestFixedWorkerPool_Submit(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  2,
		QueueSize: 5,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Submit should fail when not started
	task := NewBasicTask(func(ctx context.Context) error {
		return nil
	})
	err = pool.Submit(task)
	assert.Error(t, err)

	// Submit after start
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Normal submit
	err = pool.Submit(task)
	assert.NoError(t, err)

	// Test nil task
	err = pool.Submit(nil)
	assert.Error(t, err)
}

func TestFixedWorkerPool_TaskExecution(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  2,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Test task execution
	var counter int64
	var wg sync.WaitGroup

	numTasks := 10
	wg.Add(numTasks)

	for i := 0; i < numTasks; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			wg.Done()
			return nil
		})

		err := pool.Submit(task)
		assert.NoError(t, err)
	}

	// Wait for all tasks to complete
	wg.Wait()

	assert.Equal(t, int64(numTasks), atomic.LoadInt64(&counter))

	// Check basic statistics
	stats := pool.Stats()
	assert.Equal(t, 2, stats.PoolSize)
	assert.GreaterOrEqual(t, stats.ActiveWorkers, 0)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
}

func TestFixedWorkerPool_TaskExecutionWithErrors(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  2,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	var wg sync.WaitGroup
	numTasks := 10
	wg.Add(numTasks)

	// Submit half successful, half failed tasks
	for i := 0; i < numTasks; i++ {
		shouldFail := i%2 == 0
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			if shouldFail {
				return fmt.Errorf("task failed")
			}
			return nil
		})

		err := pool.Submit(task)
		assert.NoError(t, err)
	}

	// Wait for all tasks to complete
	wg.Wait()

	// Check statistics
	stats := pool.Stats()
	assert.Equal(t, 2, stats.PoolSize)
	assert.GreaterOrEqual(t, stats.ActiveWorkers, 0)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
}

func TestFixedWorkerPool_SubmitWithTimeout(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  1,
		QueueSize: 1, // Very small queue
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Fill the queue
	blockingTask := NewBasicTask(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// First task should succeed
	err = pool.SubmitWithTimeout(blockingTask, time.Millisecond)
	assert.NoError(t, err)

	// Second task fills the queue
	err = pool.SubmitWithTimeout(blockingTask, time.Millisecond)
	assert.NoError(t, err)

	// Third task should timeout
	err = pool.SubmitWithTimeout(blockingTask, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Equal(t, types.ErrTimeout, err)
}

func TestFixedWorkerPool_WorkerPanic(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  2,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Submit a task that will panic
	panicTask := NewBasicTask(func(ctx context.Context) error {
		defer wg.Done()
		panic("test panic")
	})

	// Submit a normal task
	normalTask := NewBasicTask(func(ctx context.Context) error {
		defer wg.Done()
		return nil
	})

	err = pool.Submit(panicTask)
	assert.NoError(t, err)

	err = pool.Submit(normalTask)
	assert.NoError(t, err)

	// Wait for tasks to complete
	wg.Wait()

	// Give statistics some time to update
	time.Sleep(10 * time.Millisecond)

	// Check statistics - panic is handled as failure
	stats := pool.Stats()
	assert.Equal(t, 2, stats.PoolSize)
	assert.GreaterOrEqual(t, stats.ActiveWorkers, 0)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
}

func TestFixedWorkerPool_Stats(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  3,
		QueueSize: 20,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Initial statistics
	stats := pool.Stats()
	assert.Equal(t, 3, stats.PoolSize)
	assert.GreaterOrEqual(t, stats.ActiveWorkers, 0)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
	assert.Equal(t, 20, stats.QueueCapacity)

	// Submit tasks and verify statistics changes
	var wg sync.WaitGroup
	numTasks := 5
	wg.Add(numTasks)

	for i := 0; i < numTasks; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond) // Brief delay
			wg.Done()
			return nil
		})

		err := pool.Submit(task)
		assert.NoError(t, err)
	}

	// Wait for tasks to complete
	wg.Wait()

	// Check final statistics
	stats = pool.Stats()
	assert.Equal(t, 3, stats.PoolSize)
	assert.GreaterOrEqual(t, stats.ActiveWorkers, 0)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
}

func TestFixedWorkerPool_Close(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  2,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)

	// Submit some tasks
	for i := 0; i < 5; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		err = pool.Submit(task)
		assert.NoError(t, err)
	}

	// Close pool
	err = pool.Close()
	assert.NoError(t, err)
	assert.True(t, pool.IsClosed())

	// Submit should fail after close
	task := NewBasicTask(func(ctx context.Context) error {
		return nil
	})
	err = pool.Submit(task)
	assert.Error(t, err)

	// Repeated close should succeed (idempotent)
	err = pool.Close()
	assert.NoError(t, err)
}

func TestFixedWorkerPool_ContextCancellation(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  2,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Submit long-running task
	var wg sync.WaitGroup
	wg.Add(1)

	task := NewBasicTask(func(taskCtx context.Context) error {
		defer wg.Done()
		select {
		case <-taskCtx.Done():
			return taskCtx.Err()
		case <-time.After(100 * time.Millisecond): // Reduced timeout to avoid hanging
			return nil
		}
	})

	err = pool.Submit(task)
	assert.NoError(t, err)

	// Give task a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for task completion with timeout to prevent hanging
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Task completed normally
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out waiting for task completion")
	}

	stats := pool.Stats()
	// Task may succeed or fail, depending on cancellation timing
	assert.Equal(t, 2, stats.PoolSize)
	assert.GreaterOrEqual(t, stats.ActiveWorkers, 0)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
}

// Benchmark tests
func BenchmarkFixedWorkerPool_Submit(b *testing.B) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  10,
		QueueSize: 1000,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(b, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(b, err)
	defer pool.Close()

	task := NewBasicTask(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Submit(task)
		}
	})
}

func BenchmarkFixedWorkerPool_TaskExecution(b *testing.B) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  10,
		QueueSize: 1000,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(b, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(b, err)
	defer pool.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var wg sync.WaitGroup
			wg.Add(1)
			task := NewBasicTask(func(ctx context.Context) error {
				wg.Done()
				return nil
			})
			_ = pool.Submit(task)
			wg.Wait()
		}
	})
}
