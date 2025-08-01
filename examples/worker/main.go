package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jzx17/gopipeline/pkg/worker"
)

func main() {
	// Create fixed-size Worker Pool configuration
	config := &worker.FixedWorkerPoolConfig{
		PoolSize:      5,   // 5 worker goroutines
		QueueSize:     100, // Queue capacity 100
		SubmitTimeout: 5 * time.Second,
	}

	// Create Worker Pool
	pool, err := worker.NewFixedWorkerPool(config)
	if err != nil {
		log.Fatalf("Failed to create worker pool: %v", err)
	}

	// Start Worker Pool
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Close()

	fmt.Printf("Worker Pool started with %d workers\n", pool.Size())

	// Example 1: Submit simple tasks
	fmt.Println("\n--- Example 1: Basic Task Execution ---")
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskID := i
		task := worker.NewBasicTaskWithID(fmt.Sprintf("task-%d", taskID), func(ctx context.Context) error {
			defer wg.Done()
			fmt.Printf("Executing task %d\n", taskID)
			time.Sleep(100 * time.Millisecond) // Simulate work
			return nil
		})

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit task %d: %v", taskID, err)
			wg.Done()
		}
	}

	wg.Wait()
	fmt.Println("All basic tasks completed")

	// Example 2: Tasks with priority
	fmt.Println("\n--- Example 2: Priority Tasks ---")

	// High priority task (higher number = higher priority)
	highPriorityTask := worker.NewBasicTaskWithPriority(func(ctx context.Context) error {
		fmt.Println("Executing HIGH priority task")
		time.Sleep(50 * time.Millisecond)
		return nil
	}, 10)

	// Low priority task
	lowPriorityTask := worker.NewBasicTaskWithPriority(func(ctx context.Context) error {
		fmt.Println("Executing LOW priority task")
		time.Sleep(50 * time.Millisecond)
		return nil
	}, 1)

	pool.Submit(lowPriorityTask)
	pool.Submit(highPriorityTask)

	time.Sleep(200 * time.Millisecond)

	// Example 3: Error handling
	fmt.Println("\n--- Example 3: Error Handling ---")

	errorTask := worker.NewBasicTask(func(ctx context.Context) error {
		return fmt.Errorf("simulated task error")
	})

	if err := pool.Submit(errorTask); err != nil {
		log.Printf("Failed to submit error task: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Example 4: Task submission with timeout
	fmt.Println("\n--- Example 4: Timeout Submission ---")

	timeoutTask := worker.NewBasicTask(func(ctx context.Context) error {
		fmt.Println("Executing timeout task")
		return nil
	})

	if err := pool.SubmitWithTimeout(timeoutTask, 1*time.Second); err != nil {
		log.Printf("Failed to submit task with timeout: %v", err)
	} else {
		fmt.Println("Task submitted successfully with timeout")
	}

	time.Sleep(100 * time.Millisecond)

	// Example 5: Get statistics
	fmt.Println("\n--- Example 5: Statistics ---")

	stats := pool.Stats()
	fmt.Printf("Pool Statistics:\n")
	fmt.Printf("  Pool Size: %d\n", stats.PoolSize)
	fmt.Printf("  Active Workers: %d\n", stats.ActiveWorkers)
	fmt.Printf("  Queue Size: %d\n", stats.QueueSize)
	fmt.Printf("  Queue Capacity: %d\n", stats.QueueCapacity)

	// Worker detailed statistics
	workerStats := pool.GetWorkerStats()
	fmt.Printf("\nWorker Details:\n")
	for _, ws := range workerStats {
		fmt.Printf("  Worker %d: State=%s, Processed=%d, Failed=%d, Success Rate=%.2f%%\n",
			ws.ID, ws.State, ws.TotalProcessed, ws.TotalFailed, ws.GetSuccessRate()*100)
	}

	// Example 6: Context cancellation
	fmt.Println("\n--- Example 6: Context Cancellation ---")

	ctx, cancel := context.WithCancel(context.Background())

	cancelableTask := worker.NewBasicTask(func(taskCtx context.Context) error {
		select {
		case <-taskCtx.Done():
			fmt.Println("Task was cancelled")
			return taskCtx.Err()
		case <-time.After(500 * time.Millisecond):
			fmt.Println("Task completed normally")
			return nil
		}
	})

	pool.Submit(cancelableTask)

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	time.Sleep(200 * time.Millisecond)

	fmt.Println("\n--- Examples Complete ---")
	fmt.Printf("Final Queue Length: %d\n", pool.QueueLength())
	fmt.Printf("Pool Running: %t\n", pool.IsRunning())
}
