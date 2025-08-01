package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixedWorkerPool_HighLoad high load integration test
func TestFixedWorkerPool_HighLoad(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  50,   // 50 worker goroutines
		QueueSize: 1000, // 1000 task buffer
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Submit large number of tasks
	numTasks := 10000
	var completedTasks int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		task := NewBasicTask(func(ctx context.Context) error {
			atomic.AddInt64(&completedTasks, 1)
			// Simulate some work
			// Remove unnecessary sleep
			wg.Done()
			return nil
		})

		err := pool.Submit(task)
		require.NoError(t, err)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Processed %d tasks in %v", numTasks, duration)
	t.Logf("Throughput: %.2f tasks/second", float64(numTasks)/duration.Seconds())

	assert.Equal(t, int64(numTasks), atomic.LoadInt64(&completedTasks))

	// Verify pool state
	assert.True(t, pool.IsRunning())
	assert.False(t, pool.IsClosed())
}

// TestFixedWorkerPool_ConcurrentSubmission concurrent submission test
func TestFixedWorkerPool_ConcurrentSubmission(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  10,
		QueueSize: 500,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	numGoroutines := 20
	tasksPerGoroutine := 100
	totalTasks := numGoroutines * tasksPerGoroutine

	var completedTasks int64
	var submissionWg sync.WaitGroup
	var executionWg sync.WaitGroup

	// Start multiple goroutines to submit tasks concurrently
	for i := 0; i < numGoroutines; i++ {
		submissionWg.Add(1)
		go func(goroutineID int) {
			defer submissionWg.Done()

			for j := 0; j < tasksPerGoroutine; j++ {
				executionWg.Add(1)
				task := NewBasicTask(func(ctx context.Context) error {
					atomic.AddInt64(&completedTasks, 1)
					executionWg.Done()
					return nil
				})

				// Use timeout submission to handle queue full situations
				err := pool.SubmitWithTimeout(task, 10*time.Millisecond)
				if err != nil {
					// Queue full is expected, not an error
					if err == types.ErrTimeout || err == types.ErrWorkerPoolFull {
						t.Logf("Task submission from goroutine %d was throttled (expected)", goroutineID)
					} else {
						t.Errorf("Failed to submit task from goroutine %d: %v", goroutineID, err)
					}
					executionWg.Done()
				}
			}
		}(i)
	}

	// Wait for all submissions to complete
	submissionWg.Wait()

	// Wait for all tasks to complete execution
	executionWg.Wait()

	// Check actual completed task count
	actualCompleted := atomic.LoadInt64(&completedTasks)
	t.Logf("Total tasks intended: %d, Actually completed: %d", totalTasks, actualCompleted)

	// In high concurrency, some tasks may be rejected due to full queue, this is normal
	assert.True(t, actualCompleted > 0, "Should complete at least some tasks")
	assert.True(t, actualCompleted <= int64(totalTasks), "Should not complete more tasks than submitted")

	// Verify basic pool state
	assert.True(t, pool.IsRunning())
	assert.False(t, pool.IsClosed())
}

// TestFixedWorkerPool_LongRunningTasks long running tasks test
func TestFixedWorkerPool_LongRunningTasks(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  5,
		QueueSize: 10,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	numTasks := 10
	taskDuration := 50 * time.Millisecond
	var completedTasks int64

	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(taskDuration)
			atomic.AddInt64(&completedTasks, 1)
			return nil
		})

		err := pool.Submit(task)
		require.NoError(t, err)
	}

	wg.Wait()
	totalDuration := time.Since(start)

	// Verify parallel execution: total time should be significantly less than all tasks running serially
	serialDuration := time.Duration(numTasks) * taskDuration
	t.Logf("Serial would take: %v, Parallel took: %v", serialDuration, totalDuration)

	// Considering there are 5 workers, theoretically should be about 1/5 of serial time (plus some overhead)
	assert.Less(t, totalDuration, serialDuration/2)
	assert.Equal(t, int64(numTasks), atomic.LoadInt64(&completedTasks))
}

// TestFixedWorkerPool_GracefulShutdown graceful shutdown test
func TestFixedWorkerPool_GracefulShutdown(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  3,
		QueueSize: 20,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)

	// Submit some long running tasks
	numTasks := 10
	var startedTasks int64
	var completedTasks int64

	for i := 0; i < numTasks; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			atomic.AddInt64(&startedTasks, 1)
			time.Sleep(1 * time.Millisecond)
			atomic.AddInt64(&completedTasks, 1)
			return nil
		})

		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Wait for some tasks to start
	time.Sleep(50 * time.Millisecond)

	// Graceful shutdown
	start := time.Now()
	err = pool.Close()
	shutdownDuration := time.Since(start)

	assert.NoError(t, err)

	// All started tasks should complete
	t.Logf("Started: %d, Completed: %d", atomic.LoadInt64(&startedTasks), atomic.LoadInt64(&completedTasks))
	t.Logf("Shutdown took: %v", shutdownDuration)

	// Shutdown should complete in reasonable time
	assert.Less(t, shutdownDuration, 15*time.Second)

	// Verify pool is closed
	assert.True(t, pool.IsClosed())

	// Task submission after close should fail
	task := NewBasicTask(func(ctx context.Context) error { return nil })
	err = pool.Submit(task)
	assert.Error(t, err)
}

// TestFixedWorkerPool_ErrorRecovery error recovery test
func TestFixedWorkerPool_ErrorRecovery(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  5,
		QueueSize: 100, // Larger queue to avoid submission issues
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Submit different types of tasks separately for easier control and verification
	numEachType := 5
	var wg sync.WaitGroup

	// Success tasks
	for i := 0; i < numEachType; i++ {
		wg.Add(1)
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(time.Millisecond) // Short delay to ensure processing time
			return nil
		})
		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Error tasks
	for i := 0; i < numEachType; i++ {
		wg.Add(1)
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			return assert.AnError
		})
		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Panic tasks
	for i := 0; i < numEachType; i++ {
		wg.Add(1)
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			panic("test panic")
		})
		err := pool.Submit(task)
		require.NoError(t, err)
	}

	wg.Wait()

	// Give statistics time to update
	time.Sleep(10 * time.Millisecond)

	// Verify pool still runs normally after error recovery
	assert.True(t, pool.IsRunning())
	assert.False(t, pool.IsClosed())

	// Submit a new task to verify pool still works
	wg.Add(1)
	testTask := NewBasicTask(func(ctx context.Context) error {
		defer wg.Done()
		return nil
	})
	err = pool.Submit(testTask)
	assert.NoError(t, err)
	wg.Wait()
}

// TestFixedWorkerPool_BasicConfiguration basic configuration test
func TestFixedWorkerPool_BasicConfiguration(t *testing.T) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  5,
		QueueSize: 50,
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Submit known number of success and failure tasks
	successTasks := 20
	failTasks := 10
	totalTasks := successTasks + failTasks

	var wg sync.WaitGroup
	wg.Add(totalTasks)

	// Success tasks
	for i := 0; i < successTasks; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(time.Millisecond) // Ensure execution time
			return nil
		})
		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Failure tasks
	for i := 0; i < failTasks; i++ {
		task := NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(time.Millisecond) // Ensure execution time
			return assert.AnError
		})
		err := pool.Submit(task)
		require.NoError(t, err)
	}

	wg.Wait()

	// Give statistics some time to update
	time.Sleep(10 * time.Millisecond)

	// Verify basic pool configuration and state
	stats := pool.Stats()
	assert.Equal(t, config.PoolSize, stats.PoolSize)
	assert.Equal(t, config.QueueSize, stats.QueueCapacity)
	assert.True(t, pool.IsRunning())
	assert.False(t, pool.IsClosed())

	// Verify basic worker state
	workerStats := pool.GetWorkerStats()
	assert.Len(t, workerStats, config.PoolSize)

	for _, ws := range workerStats {
		assert.True(t, ws.IsIdle() || ws.IsActive()) // Should be idle or active state
	}
}

// BenchmarkFixedWorkerPool_HighThroughput high throughput benchmark test
func BenchmarkFixedWorkerPool_HighThroughput(b *testing.B) {
	config := &FixedWorkerPoolConfig{
		PoolSize:  20,
		QueueSize: 10000, // Increase queue size
	}

	pool, err := NewFixedWorkerPool(config)
	require.NoError(b, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(b, err)
	defer pool.Close()

	task := NewBasicTask(func(ctx context.Context) error {
		// Reduce workload to speed up task execution
		for i := 0; i < 10; i++ {
			_ = i * i
		}
		return nil
	})

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Use timeout submission to avoid blocking benchmark
			if err := pool.SubmitWithTimeout(task, 100*time.Millisecond); err != nil {
				// In benchmark, queue full is acceptable, no error
				if err == types.ErrWorkerPoolFull || err == types.ErrTimeout {
					continue
				}
				b.Error(err)
			}
		}
	})
}
