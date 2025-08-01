// Package main demonstrates asynchronous Pipeline usage
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jzx17/gopipeline/pkg/pipeline"
	"github.com/jzx17/gopipeline/pkg/types"
)

func main() {
	fmt.Println("=== Asynchronous Pipeline Example ===")

	// Create Pipeline configuration
	config := &types.Config{
		Workers:      5,
		BufferSize:   100,
		Timeout:      30 * time.Second,
		StageTimeout: 5 * time.Second,
		MaxRetries:   3,
		RetryDelay:   100 * time.Millisecond,
	}

	// Create processing function (multiply number by 2)
	processFunc := func(ctx context.Context, input string) (int, error) {
		// Simulate some processing delay
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return 0, ctx.Err()
		}

		num, err := strconv.Atoi(input)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", input)
		}

		return num * 2, nil
	}

	// Create Pipeline
	p := pipeline.New[string, int](processFunc, config)

	// Start Pipeline
	if err := p.Start(); err != nil {
		log.Fatalf("Failed to start Pipeline: %v", err)
	}
	defer p.Close()

	// Demo 1: Basic async execution
	fmt.Println("\n1. Basic Async Execution:")
	demonstrateBasicAsync(p)

	// Demo 2: Batch async processing
	fmt.Println("\n2. Batch Async Processing:")
	demonstrateBatchAsync(p)

	// Demo 3: Using AsyncPipeline wrapper
	fmt.Println("\n3. AsyncPipeline Wrapper:")
	demonstrateAsyncWrapper(p)
}

// demonstrateBasicAsync demonstrates basic async execution
func demonstrateBasicAsync(p types.Pipeline[string, int]) {
	ctx := context.Background()

	// Async execution
	resultChan := p.ExecuteAsync(ctx, "42")

	// Wait for result
	result := <-resultChan
	if result.Error != nil {
		fmt.Printf("Error: %v\n", result.Error)
	} else {
		fmt.Printf("Result: %d, Duration: %v\n", result.Value, result.Duration)
	}
}

// demonstrateBatchAsync demonstrates batch async processing
func demonstrateBatchAsync(p types.Pipeline[string, int]) {
	ctx := context.Background()

	// Prepare batch data
	inputs := make([]string, 10)
	for i := 0; i < 10; i++ {
		inputs[i] = fmt.Sprintf("%d", i+1)
	}

	// Batch processing
	start := time.Now()
	resultChan := p.ExecuteBatch(ctx, inputs)

	// Collect results
	results := make(map[int]int)
	var errors int

	for result := range resultChan {
		if result.Error != nil {
			errors++
			fmt.Printf("Processing error at index %d: %v\n", result.Index, result.Error)
		} else {
			results[result.Index] = result.Value
		}
	}

	duration := time.Since(start)
	fmt.Printf("Batch processing complete, %d successful, %d errors, total duration: %v\n",
		len(results), errors, duration)

	// Display results in index order
	fmt.Print("Results: [")
	for i := 0; i < len(inputs); i++ {
		if val, ok := results[i]; ok {
			fmt.Printf("%d", val)
		} else {
			fmt.Print("error")
		}
		if i < len(inputs)-1 {
			fmt.Print(", ")
		}
	}
	fmt.Println("]")
}

// demonstrateAsyncWrapper demonstrates AsyncPipeline wrapper
func demonstrateAsyncWrapper(p types.Pipeline[string, int]) {
	ctx := context.Background()

	// Create async Pipeline wrapper
	asyncPipeline := pipeline.NewAsyncPipeline(p, &pipeline.AsyncOptions{
		BufferSize:  20,
		Concurrency: 4,
	})
	defer asyncPipeline.Close()

	// Use wrapper for async execution
	fmt.Println("Using AsyncPipeline wrapper:")
	resultChan := asyncPipeline.ExecuteAsync(ctx, "123")
	result := <-resultChan
	if result.Error != nil {
		fmt.Printf("Error: %v\n", result.Error)
	} else {
		fmt.Printf("Wrapper result: %d, Duration: %v\n", result.Value, result.Duration)
	}

	// Use wrapper for batch processing
	fmt.Println("Using wrapper for batch processing:")
	inputs := []string{"10", "20", "30", "40", "50"}
	start := time.Now()
	batchResultChan := asyncPipeline.ExecuteBatch(ctx, inputs)

	var processedCount int
	for result := range batchResultChan {
		if result.Error != nil {
			fmt.Printf("Batch processing error [%d]: %v\n", result.Index, result.Error)
		} else {
			fmt.Printf("Batch result [%d]: %d\n", result.Index, result.Value)
			processedCount++
		}
	}

	duration := time.Since(start)
	fmt.Printf("Wrapper batch processing complete, processed %d items, total duration: %v\n", processedCount, duration)

	// Demonstrate error handling
	fmt.Println("Demonstrating error handling:")
	errorResultChan := asyncPipeline.ExecuteAsync(ctx, "invalid")
	errorResult := <-errorResultChan
	if errorResult.Error != nil {
		fmt.Printf("Expected error: %v\n", errorResult.Error)
	}
}
