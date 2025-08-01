package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jzx17/gopipeline/pkg/worker"
)

// Example task
type PrintTask struct {
	id       string
	priority int
	message  string
}

func NewPrintTask(id, message string, priority int) *PrintTask {
	return &PrintTask{
		id:       id,
		priority: priority,
		message:  message,
	}
}

func (t *PrintTask) Execute(ctx context.Context) error {
	fmt.Printf("[Priority %d] Task %s: %s\n", t.priority, t.id, t.message)
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

func (t *PrintTask) ID() string {
	return t.id
}

func (t *PrintTask) Priority() int {
	return t.priority
}

func main() {
	fmt.Println("=== Go-Pipeline Priority Worker Pool Example ===")

	// Create priority worker pool configuration
	config := worker.DefaultPriorityWorkerPoolConfig()
	config.PoolSize = 2 // Use 2 workers
	config.QueueCapacity = 20

	// Enable starvation protection
	config.StarvationConfig = &worker.StarvationConfig{
		MaxWaitTime:          2 * time.Second,
		AgePromotionInterval: 500 * time.Millisecond,
		PriorityBoostAmount:  1,
		MaxPriorityBoost:     5,
		Enable:               true,
	}

	// Create priority worker pool
	pool, err := worker.NewPriorityWorkerPool(config)
	if err != nil {
		log.Fatalf("Failed to create priority worker pool: %v", err)
	}

	// Start the worker pool
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Close()

	fmt.Println("1. Basic Priority Demonstration")
	fmt.Println("   Submit tasks with different priorities and observe execution order")

	// Submit tasks with different priorities
	tasks := []struct {
		id       string
		message  string
		priority int
	}{
		{"T1", "Low priority task", 1},
		{"T2", "Medium priority task", 5},
		{"T3", "High priority task", 10},
		{"T4", "Ultra high priority task", 15},
		{"T5", "Another low priority task", 2},
		{"T6", "Another high priority task", 12},
	}

	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		printTask := NewPrintTask(task.id, task.message, task.priority)

		// Submit task with priority
		if err := pool.SubmitWithPriority(printTask, task.priority); err != nil {
			log.Printf("Failed to submit task: %v", err)
			wg.Done()
			continue
		}

		// For demonstration, we use goroutine to wait, but this is not needed in actual use
		go func() {
			defer wg.Done()
			// This is just for demonstration waiting, tasks will execute automatically in practice
		}()
	}

	// Wait for a while to let tasks execute
	time.Sleep(2 * time.Second)

	fmt.Printf("\n2. Priority Statistics\n")
	stats := pool.GetPriorityStats()
	fmt.Printf("   Current waiting tasks: %d\n", stats.TotalWaitingTasks)
	fmt.Printf("   Average wait time: %v\n", stats.AverageWaitTime)
	fmt.Printf("   Max wait time: %v\n", stats.MaxWaitTime)
	fmt.Printf("   Age promotions: %d\n", stats.AgePromotions)
	fmt.Printf("   Starvation detected: %d\n", stats.StarvationDetected)

	if len(stats.QueueLengthByPriority) > 0 {
		fmt.Printf("   Queue length by priority:\n")
		for priority, length := range stats.QueueLengthByPriority {
			fmt.Printf("     Priority %d: %d tasks\n", priority, length)
		}
	}

	fmt.Printf("\n3. Worker Pool Basic Statistics\n")
	poolStats := pool.Stats()
	fmt.Printf("   Pool size: %d\n", poolStats.PoolSize)
	fmt.Printf("   Active workers: %d\n", poolStats.ActiveWorkers)
	fmt.Printf("   Current queue size: %d\n", poolStats.QueueSize)
	fmt.Printf("   Queue capacity: %d\n", poolStats.QueueCapacity)

	fmt.Printf("\n4. Starvation Protection Demo\n")
	fmt.Println("   Submit a low priority task, then continuously submit high priority tasks")
	fmt.Println("   Observe if low priority task gets priority boost due to starvation protection")

	// Submit a low priority long-running task
	starvationTask := NewPrintTask("STARV", "Low priority task that might starve", 1)
	if err := pool.SubmitWithPriority(starvationTask, 1); err != nil {
		log.Printf("Failed to submit starvation test task: %v", err)
	}

	// Continuously submit high priority tasks
	for i := 0; i < 5; i++ {
		highPriorityTask := NewPrintTask(
			fmt.Sprintf("HIGH%d", i),
			fmt.Sprintf("High priority task #%d", i),
			10,
		)
		if err := pool.SubmitWithPriority(highPriorityTask, 10); err != nil {
			log.Printf("Failed to submit high priority task: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Wait for starvation protection to take effect
	fmt.Println("   Waiting for starvation protection mechanism to take effect...")
	time.Sleep(3 * time.Second)

	fmt.Printf("\n5. Final Statistics\n")
	finalStats := pool.GetPriorityStats()
	fmt.Printf("   Age promotions: %d\n", finalStats.AgePromotions)
	fmt.Printf("   Starvation detected: %d\n", finalStats.StarvationDetected)

	finalPoolStats := pool.Stats()
	fmt.Printf("   Current active workers: %d\n", finalPoolStats.ActiveWorkers)

	fmt.Printf("\n6. Dynamic Priority Adjustment Demo\n")

	// Submit a task
	adjustTask := NewPrintTask("ADJ", "Task with adjustable priority", 1)
	if err := pool.SubmitWithPriority(adjustTask, 1); err != nil {
		log.Printf("Failed to submit adjustable task: %v", err)
	} else {
		fmt.Printf("   Submitted task with priority 1: %s\n", adjustTask.ID())

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Try to adjust priority
		if err := pool.UpdatePriority(adjustTask.ID(), 15); err != nil {
			log.Printf("Failed to adjust task priority: %v", err)
		} else {
			fmt.Printf("   Successfully adjusted task %s priority to 15\n", adjustTask.ID())
		}
	}

	// Wait for all tasks to complete
	time.Sleep(2 * time.Second)

	fmt.Println("\n=== Demo Complete ===")
}
