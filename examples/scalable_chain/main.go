package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jzx17/gopipeline/pkg/pipeline"
)

func main() {
	fmt.Println("=== Scalable Chain Pipeline Example ===\n")

	ctx := context.Background()

	// Demonstrate solution 1: Dynamic chain composition (recommended)
	demonstrateDynamicChain(ctx)

	// Demonstrate solution 2: Chain builder
	demonstrateChainBuilder(ctx)

	// Demonstrate solution 4: Array-driven chain
	demonstrateArrayChain(ctx)

	// Demonstrate solution 5: Immutable chain
	demonstrateImmutableChain(ctx)

	// Performance comparison
	performanceComparison(ctx)
}

func demonstrateDynamicChain(ctx context.Context) {
	fmt.Println("--- Solution 1: Dynamic Chain Composition ---")

	// Create a processing chain with 10 steps
	chain := pipeline.NewDynamicChain[string]().
		Then(func(ctx context.Context, input string) (string, error) {
			return "step1:" + input, nil
		}).
		Then(func(ctx context.Context, input string) (string, error) {
			return "step2:" + input, nil
		}).
		Then(func(ctx context.Context, input string) (string, error) {
			return "step3:" + input, nil
		}).
		Then(func(ctx context.Context, input string) (string, error) {
			return "step4:" + input, nil
		}).
		Then(func(ctx context.Context, input string) (string, error) {
			return "step5:" + input, nil
		})

	result, err := chain.Execute(ctx, "input")
	if err != nil {
		log.Printf("Dynamic chain execution failed: %v", err)
		return
	}
	fmt.Printf("Dynamic chain result: %s\n\n", result)
}

func demonstrateChainBuilder(ctx context.Context) {
	fmt.Println("--- Solution 2: Chain Builder ---")

	builder := pipeline.NewChainBuilder[int]()

	// Add multiple processing steps
	steps := []struct {
		name string
		fn   pipeline.ProcessFunc[int, int]
	}{
		{"multiply_by_2", func(ctx context.Context, n int) (int, error) { return n * 2, nil }},
		{"add_10", func(ctx context.Context, n int) (int, error) { return n + 10, nil }},
		{"multiply_by_3", func(ctx context.Context, n int) (int, error) { return n * 3, nil }},
		{"subtract_5", func(ctx context.Context, n int) (int, error) { return n - 5, nil }},
		{"divide_by_2", func(ctx context.Context, n int) (int, error) { return n / 2, nil }},
	}

	for _, step := range steps {
		builder.AddFunc(step.name, step.fn)
	}

	process := builder.Build()
	result, err := process(ctx, 5)
	if err != nil {
		log.Printf("Chain builder execution failed: %v", err)
		return
	}
	fmt.Printf("Chain builder result: %d (5 -> *2 -> +10 -> *3 -> -5 -> /2 = %d)\n\n", result, result)
}

func demonstrateArrayChain(ctx context.Context) {
	fmt.Println("--- Solution 4: Array-Driven Chain ---")

	// Create string processing chain
	chain := pipeline.NewArrayChain(
		func(ctx context.Context, s string) (string, error) {
			return strings.ToUpper(s), nil
		},
		func(ctx context.Context, s string) (string, error) {
			return strings.ReplaceAll(s, " ", "_"), nil
		},
		func(ctx context.Context, s string) (string, error) {
			return "PROCESSED_" + s, nil
		},
	)

	// Dynamically add more steps
	chain = chain.Append(func(ctx context.Context, s string) (string, error) {
		return s + "_FINAL", nil
	})

	result, err := chain.Execute(ctx, "hello world")
	if err != nil {
		log.Printf("Array chain execution failed: %v", err)
		return
	}
	fmt.Printf("Array chain result: %s\n\n", result)
}

func demonstrateImmutableChain(ctx context.Context) {
	fmt.Println("--- Solution 5: Immutable Chain ---")

	// Create data transformation pipeline
	baseChain := pipeline.NewImmutableChain(
		func(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
			data["processed"] = true
			return data, nil
		},
		func(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
			data["timestamp"] = time.Now().Unix()
			return data, nil
		},
	)

	// Extend chain (without modifying original chain)
	extendedChain := baseChain.
		Then(func(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
			data["validated"] = true
			return data, nil
		}).
		Then(func(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
			data["final_step"] = "completed"
			return data, nil
		})

	input := map[string]interface{}{
		"id":   "12345",
		"name": "test_data",
	}

	result, err := extendedChain.Execute(ctx, input)
	if err != nil {
		log.Printf("Immutable chain execution failed: %v", err)
		return
	}
	fmt.Printf("Immutable chain result: %+v\n\n", result)
}

func performanceComparison(ctx context.Context) {
	fmt.Println("--- Performance Comparison ---")

	const iterations = 100000
	input := "test"

	// Test function
	stepFunc := func(ctx context.Context, s string) (string, error) {
		return s + "x", nil
	}

	// Solution 1: Dynamic chain
	dynamicChain := pipeline.NewDynamicChain[string]().
		Then(stepFunc).Then(stepFunc).Then(stepFunc).Then(stepFunc).Then(stepFunc)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = dynamicChain.Execute(ctx, input)
	}
	fmt.Printf("Dynamic chain (%d iterations): %v\n", iterations, time.Since(start))

	// Solution 4: Array chain
	arrayChain := pipeline.NewArrayChain(stepFunc, stepFunc, stepFunc, stepFunc, stepFunc)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = arrayChain.Execute(ctx, input)
	}
	fmt.Printf("Array chain (%d iterations): %v\n", iterations, time.Since(start))

	// Solution 5: Immutable chain
	immutableChain := pipeline.NewImmutableChain(stepFunc, stepFunc, stepFunc, stepFunc, stepFunc)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = immutableChain.Execute(ctx, input)
	}
	fmt.Printf("Immutable chain (%d iterations): %v\n", iterations, time.Since(start))

	// ImmutableChain comparison
	immutableChain2 := pipeline.NewImmutableChain(stepFunc, stepFunc, stepFunc, stepFunc)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = immutableChain2.Execute(ctx, input)
	}
	fmt.Printf("ImmutableChain2 (%d iterations): %v\n", iterations, time.Since(start))
}
