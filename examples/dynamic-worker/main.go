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
	fmt.Println("=== Dynamic Worker Pool Example ===")

	// Create dynamic worker pool configuration
	config := &worker.DynamicWorkerPoolConfig{
		MinWorkers:    2,
		MaxWorkers:    8,
		IdleTimeout:   30 * time.Second,
		QueueSize:     50,
		SubmitTimeout: 5 * time.Second,
	}

	// Create dynamic worker pool
	pool, err := worker.NewDynamicWorkerPool(config)
	if err != nil {
		log.Fatalf("Failed to create dynamic worker pool: %v", err)
	}

	// Start the worker pool
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Close()

	fmt.Printf("Dynamic worker pool started, initial size: %d workers\n", pool.Size())

	// Example 1: Basic task execution
	fmt.Println("\n--- Example 1: Basic Task Execution ---")
	executeBasicTasks(pool)

	// Example 2: Batch task processing
	fmt.Println("\n--- Example 2: Batch Task Processing ---")
	executeBatchTasks(pool)

	// Example 3: Long-running tasks
	fmt.Println("\n--- Example 3: Long-Running Tasks ---")
	executeLongRunningTasks(pool)

	// Show final statistics
	printFinalStats(pool)
}

// executeBasicTasks executes basic tasks
func executeBasicTasks(pool *worker.DynamicWorkerPool) {
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskID := i
		task := worker.NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			fmt.Printf("Executing basic task %d\n", taskID)
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
	printStats(pool)
}

// executeBatchTasks executes batch tasks
func executeBatchTasks(pool *worker.DynamicWorkerPool) {
	var wg sync.WaitGroup
	taskCount := 20

	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		taskID := i
		task := worker.NewBasicTaskWithID(fmt.Sprintf("batch-task-%d", taskID), func(ctx context.Context) error {
			defer wg.Done()
			fmt.Printf("Batch task %d starting execution\n", taskID)

			// Simulate different processing times
			duration := time.Duration(50+taskID*10) * time.Millisecond
			select {
			case <-time.After(duration):
				fmt.Printf("Batch task %d completed\n", taskID)
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit batch task %d: %v", taskID, err)
			wg.Done()
		}
	}

	// Display statistics every second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 5; i++ {
			<-ticker.C
			stats := pool.Stats()
			fmt.Printf("[Stats] Workers: %d, Active: %d, Queue: %d/%d\n",
				stats.PoolSize, stats.ActiveWorkers, stats.QueueSize, stats.QueueCapacity)
		}
	}()

	wg.Wait()
	fmt.Println("All batch tasks completed")
	printStats(pool)
}

// executeLongRunningTasks executes long-running tasks
func executeLongRunningTasks(pool *worker.DynamicWorkerPool) {
	var wg sync.WaitGroup

	// Submit several long-running tasks
	for i := 0; i < 3; i++ {
		wg.Add(1)
		taskID := i
		task := worker.NewBasicTask(func(ctx context.Context) error {
			defer wg.Done()
			fmt.Printf("Long-running task %d starting\n", taskID)

			select {
			case <-time.After(2 * time.Second):
				fmt.Printf("Long-running task %d completed\n", taskID)
				return nil
			case <-ctx.Done():
				fmt.Printf("Long-running task %d was cancelled\n", taskID)
				return ctx.Err()
			}
		})

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit long-running task %d: %v", taskID, err)
			wg.Done()
		}
	}

	wg.Wait()
	fmt.Println("All long-running tasks completed")
	printStats(pool)
}

// printStats prints current statistics
func printStats(pool *worker.DynamicWorkerPool) {
	stats := pool.Stats()
	fmt.Printf("Current stats: Workers=%d, Active=%d, Queue=%d/%d\n",
		stats.PoolSize, stats.ActiveWorkers, stats.QueueSize, stats.QueueCapacity)
}

// printFinalStats prints final statistics
func printFinalStats(pool *worker.DynamicWorkerPool) {
	fmt.Println("\n--- Final Statistics ---")
	stats := pool.Stats()
	fmt.Printf("Final worker count: %d\n", stats.PoolSize)
	fmt.Printf("Active workers: %d\n", stats.ActiveWorkers)
	fmt.Printf("Queue status: %d/%d\n", stats.QueueSize, stats.QueueCapacity)
	fmt.Printf("Pool running: %t\n", pool.IsRunning())
}
