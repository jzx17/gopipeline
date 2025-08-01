package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPriorityTask tests priority task functionality
func TestPriorityTask(t *testing.T) {
	t.Run("CreatePriorityTask", func(t *testing.T) {
		basicTask := NewBasicTask(func(ctx context.Context) error {
			return nil
		})

		priorityTask := NewPriorityTaskFromTask(basicTask, 5)

		assert.Equal(t, 5, priorityTask.Priority())
		assert.Equal(t, int32(0), atomic.LoadInt32(&priorityTask.AgeBoost))
		assert.Equal(t, 5, priorityTask.GetEffectivePriority())
		assert.NotZero(t, priorityTask.SubmitTime)
	})

	t.Run("BoostPriority", func(t *testing.T) {
		basicTask := NewBasicTask(func(ctx context.Context) error {
			return nil
		})

		priorityTask := NewPriorityTaskFromTask(basicTask, 5)

		// Boost priority
		priorityTask.BoostPriority(2)
		assert.Equal(t, int32(2), atomic.LoadInt32(&priorityTask.AgeBoost))
		assert.Equal(t, 7, priorityTask.GetEffectivePriority())

		// Boost again
		priorityTask.BoostPriority(3)
		assert.Equal(t, int32(5), atomic.LoadInt32(&priorityTask.AgeBoost))
		assert.Equal(t, 10, priorityTask.GetEffectivePriority())
	})
}

// TestPriorityQueue tests priority queue functionality
func TestPriorityQueue(t *testing.T) {
	t.Run("BasicEnqueueDequeue", func(t *testing.T) {
		config := &StarvationConfig{Enable: false} // Disable starvation protection to simplify test
		pq := NewPriorityQueue(config)
		defer pq.Close()

		// Create tasks with different priorities
		tasks := []*PriorityTask{
			NewPriorityTaskFromTask(NewBasicTaskWithID("low", func(ctx context.Context) error { return nil }), 1),
			NewPriorityTaskFromTask(NewBasicTaskWithID("high", func(ctx context.Context) error { return nil }), 10),
			NewPriorityTaskFromTask(NewBasicTaskWithID("medium", func(ctx context.Context) error { return nil }), 5),
		}

		// Enqueue tasks
		for _, task := range tasks {
			pq.Enqueue(task)
		}

		assert.Equal(t, 3, pq.Size())
		assert.False(t, pq.IsEmpty())

		// Dequeue should follow priority order
		task1, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "high", task1.ID())

		task2, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "medium", task2.ID())

		task3, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "low", task3.ID())

		// Queue should be empty
		assert.Equal(t, 0, pq.Size())
		assert.True(t, pq.IsEmpty())

		_, ok = pq.Dequeue()
		assert.False(t, ok)
	})

	t.Run("FIFOForSamePriority", func(t *testing.T) {
		config := &StarvationConfig{Enable: false}
		pq := NewPriorityQueue(config)
		defer pq.Close()

		// Create tasks with same priority but different submit times
		task1 := NewPriorityTaskFromTask(NewBasicTaskWithID("first", func(ctx context.Context) error { return nil }), 5)
		time.Sleep(time.Millisecond) // Ensure time difference
		task2 := NewPriorityTaskFromTask(NewBasicTaskWithID("second", func(ctx context.Context) error { return nil }), 5)
		time.Sleep(time.Millisecond)
		task3 := NewPriorityTaskFromTask(NewBasicTaskWithID("third", func(ctx context.Context) error { return nil }), 5)

		// Enqueue in random order
		pq.Enqueue(task2)
		pq.Enqueue(task1)
		pq.Enqueue(task3)

		// Dequeue should follow FIFO order
		dequeued1, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "first", dequeued1.ID())

		dequeued2, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "second", dequeued2.ID())

		dequeued3, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "third", dequeued3.ID())
	})

	t.Run("Peek", func(t *testing.T) {
		config := &StarvationConfig{Enable: false}
		pq := NewPriorityQueue(config)
		defer pq.Close()

		// Empty queue
		_, ok := pq.Peek()
		assert.False(t, ok)

		// Add task
		task := NewPriorityTaskFromTask(NewBasicTaskWithID("test", func(ctx context.Context) error { return nil }), 5)
		pq.Enqueue(task)

		// Peek should return without removing
		peeked, ok := pq.Peek()
		require.True(t, ok)
		assert.Equal(t, "test", peeked.ID())
		assert.Equal(t, 1, pq.Size())

		// Peek again should return the same task
		peeked2, ok := pq.Peek()
		require.True(t, ok)
		assert.Equal(t, "test", peeked2.ID())
		assert.Equal(t, 1, pq.Size())
	})

	t.Run("UpdateTaskPriority", func(t *testing.T) {
		config := &StarvationConfig{Enable: false}
		pq := NewPriorityQueue(config)
		defer pq.Close()

		// Add tasks
		tasks := []*PriorityTask{
			NewPriorityTaskFromTask(NewBasicTaskWithID("task1", func(ctx context.Context) error { return nil }), 1),
			NewPriorityTaskFromTask(NewBasicTaskWithID("task2", func(ctx context.Context) error { return nil }), 2),
			NewPriorityTaskFromTask(NewBasicTaskWithID("task3", func(ctx context.Context) error { return nil }), 3),
		}

		for _, task := range tasks {
			pq.Enqueue(task)
		}

		// Update task1 priority to highest
		err := pq.UpdateTaskPriority("task1", 10)
		require.NoError(t, err)

		// Now task1 should be dequeued first
		dequeued, ok := pq.Dequeue()
		require.True(t, ok)
		assert.Equal(t, "task1", dequeued.ID())

		// Try to update non-existent task
		err = pq.UpdateTaskPriority("nonexistent", 5)
		assert.Error(t, err)
	})

	t.Run("GetStats", func(t *testing.T) {
		config := &StarvationConfig{Enable: false}
		pq := NewPriorityQueue(config)
		defer pq.Close()

		// Add tasks with different priorities
		priorities := []int{1, 1, 2, 2, 2, 3}
		for i, priority := range priorities {
			task := NewPriorityTaskFromTask(
				NewBasicTaskWithID(fmt.Sprintf("task%d", i), func(ctx context.Context) error { return nil }),
				priority,
			)
			pq.Enqueue(task)
		}

		stats := pq.GetStats()

		// Check queue length by priority
		assert.Equal(t, 2, stats.QueueLengthByPriority[1])
		assert.Equal(t, 3, stats.QueueLengthByPriority[2])
		assert.Equal(t, 1, stats.QueueLengthByPriority[3])

		// Check total waiting tasks
		assert.Equal(t, int64(6), stats.TotalWaitingTasks)
	})
}

// TestStarvationGuard tests starvation protection mechanism
func TestStarvationGuard(t *testing.T) {
	t.Run("StarvationPrevention", func(t *testing.T) {
		config := &StarvationConfig{
			MaxWaitTime:          50 * time.Millisecond,
			AgePromotionInterval: 20 * time.Millisecond,
			PriorityBoostAmount:  1,
			MaxPriorityBoost:     5,
			Enable:               true,
		}

		pq := NewPriorityQueue(config)
		defer pq.Close()

		// Add a low priority task
		lowPriorityTask := NewPriorityTaskFromTask(
			NewBasicTaskWithID("low", func(ctx context.Context) error { return nil }),
			1,
		)
		pq.Enqueue(lowPriorityTask)

		// Wait long enough for starvation protection to take effect
		time.Sleep(100 * time.Millisecond)

		// Check if task priority was boosted
		stats := pq.GetStats()
		assert.True(t, stats.AgePromotions > 0, "Age promotions should have occurred")

		// Task effective priority should be boosted
		peeked, ok := pq.Peek()
		require.True(t, ok)
		assert.True(t, peeked.GetEffectivePriority() > 1, "Task priority should be boosted")
	})

	t.Run("MaxBoostLimit", func(t *testing.T) {
		config := &StarvationConfig{
			MaxWaitTime:          20 * time.Millisecond,
			AgePromotionInterval: 10 * time.Millisecond,
			PriorityBoostAmount:  1,
			MaxPriorityBoost:     3, // Can only boost up to 3
			Enable:               true,
		}

		pq := NewPriorityQueue(config)
		defer pq.Close()

		task := NewPriorityTaskFromTask(
			NewBasicTaskWithID("test", func(ctx context.Context) error { return nil }),
			1,
		)
		pq.Enqueue(task)

		// Wait long enough for multiple boosts, but ensure it doesn't exceed limit
		time.Sleep(150 * time.Millisecond)

		// Check that task AgeBoost doesn't exceed MaxPriorityBoost
		peeked, ok := pq.Peek()
		require.True(t, ok)
		assert.LessOrEqual(t, int(atomic.LoadInt32(&peeked.AgeBoost)), 3, "Age boost should not exceed MaxPriorityBoost")
	})
}

// TestConcurrentAccess tests concurrent access
func TestConcurrentAccess(t *testing.T) {
	config := &StarvationConfig{Enable: false}
	pq := NewPriorityQueue(config)
	defer pq.Close()

	const numGoroutines = 10
	const tasksPerGoroutine = 100

	// Concurrent enqueue
	enqueueDone := make(chan struct{})
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer func() { enqueueDone <- struct{}{} }()
			for j := 0; j < tasksPerGoroutine; j++ {
				task := NewPriorityTaskFromTask(
					NewBasicTaskWithID(fmt.Sprintf("task-%d-%d", routineID, j), func(ctx context.Context) error { return nil }),
					j%10, // Priority 0-9
				)
				pq.Enqueue(task)
			}
		}(i)
	}

	// Wait for all enqueue operations to complete
	for i := 0; i < numGoroutines; i++ {
		<-enqueueDone
	}

	// Check total count
	expectedTotal := numGoroutines * tasksPerGoroutine
	assert.Equal(t, expectedTotal, pq.Size())

	// Concurrent dequeue
	dequeuedTasks := make(chan *PriorityTask, expectedTotal)
	dequeueDone := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { dequeueDone <- struct{}{} }()
			for {
				if task, ok := pq.Dequeue(); ok {
					dequeuedTasks <- task
				} else {
					break
				}
			}
		}()
	}

	// Wait for all dequeue operations to complete
	for i := 0; i < numGoroutines; i++ {
		<-dequeueDone
	}
	close(dequeuedTasks)

	// Check dequeued task count
	var dequeuedCount int
	priorityCounts := make(map[int]int)

	for task := range dequeuedTasks {
		dequeuedCount++
		// Count by priority, no strict ordering required (due to concurrency)
		priorityCounts[task.GetEffectivePriority()]++
	}

	assert.Equal(t, expectedTotal, dequeuedCount)
	assert.Equal(t, 0, pq.Size())
}

// BenchmarkPriorityQueueEnqueue benchmarks enqueue operations
func BenchmarkPriorityQueueEnqueue(b *testing.B) {
	config := &StarvationConfig{Enable: false}
	pq := NewPriorityQueue(config)
	defer pq.Close()

	// Prepare tasks
	tasks := make([]*PriorityTask, b.N)
	for i := 0; i < b.N; i++ {
		tasks[i] = NewPriorityTaskFromTask(
			NewBasicTaskWithID(fmt.Sprintf("task-%d", i), func(ctx context.Context) error { return nil }),
			i%100, // Priority 0-99
		)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pq.Enqueue(tasks[i])
	}
}

// BenchmarkPriorityQueueDequeue benchmarks dequeue operations
func BenchmarkPriorityQueueDequeue(b *testing.B) {
	config := &StarvationConfig{Enable: false}
	pq := NewPriorityQueue(config)
	defer pq.Close()

	// Fill queue first
	for i := 0; i < b.N; i++ {
		task := NewPriorityTaskFromTask(
			NewBasicTaskWithID(fmt.Sprintf("task-%d", i), func(ctx context.Context) error { return nil }),
			i%100,
		)
		pq.Enqueue(task)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, ok := pq.Dequeue(); !ok {
			b.Fatal("Failed to dequeue task")
		}
	}
}

// TestPriorityQueueMemoryUsage tests memory usage
func TestPriorityQueueMemoryUsage(t *testing.T) {
	config := &StarvationConfig{Enable: false}
	pq := NewPriorityQueue(config)
	defer pq.Close()

	const numTasks = 10000

	// Add many tasks
	for i := 0; i < numTasks; i++ {
		task := NewPriorityTaskFromTask(
			NewBasicTaskWithID(fmt.Sprintf("task-%d", i), func(ctx context.Context) error { return nil }),
			i%100,
		)
		pq.Enqueue(task)
	}

	assert.Equal(t, numTasks, pq.Size())

	// Remove all tasks
	for i := 0; i < numTasks; i++ {
		_, ok := pq.Dequeue()
		assert.True(t, ok)
	}

	assert.Equal(t, 0, pq.Size())
	assert.True(t, pq.IsEmpty())
}
