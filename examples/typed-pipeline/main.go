package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jzx17/gopipeline/pkg/pipeline"
)

func main() {
	fmt.Println("=== Functional Type-Safe Pipeline Example ===")

	// Create pipeline using scalable chain processing
	// All processing uses string type
	typeSafePipeline := pipeline.NewChainBuilder[string]().
		AddFunc("parse", func(ctx context.Context, s string) (string, error) {
			// Step 1: Parse string to integer
			i, err := strconv.Atoi(s)
			if err != nil {
				return "", err
			}
			return strconv.Itoa(i), nil
		}).
		AddFunc("double", func(ctx context.Context, s string) (string, error) {
			// Step 2: Multiply by 2
			i, _ := strconv.Atoi(s)
			doubled := i * 2
			return strconv.Itoa(doubled), nil
		}).
		AddFunc("format", func(ctx context.Context, s string) (string, error) {
			// Step 3: Format result
			i, _ := strconv.Atoi(s)
			return fmt.Sprintf("result: %d", i), nil
		}).
		Build()

	// Execute pipeline
	ctx := context.Background()

	// Test inputs
	inputs := []string{"10", "25", "42", "invalid", "99"}

	for _, input := range inputs {
		fmt.Printf("\nProcessing input: %s\n", input)

		result, err := typeSafePipeline(ctx, input)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  Result: %v (type: %T)\n", result, result)
		}
	}

	fmt.Println("\n=== Functional vs Legacy Implementation Comparison ===")
	fmt.Println("Issues with legacy implementation:")
	fmt.Println("1. Uses dangerous type assertion any(result).(T)")
	fmt.Println("2. Assumes all stages have the same input/output types")
	fmt.Println("3. Type mismatches only discovered at runtime")
	fmt.Println("4. Requires extensive builder code and reflection")
	fmt.Println("")
	fmt.Println("Advantages of functional implementation:")
	fmt.Println("1. Compile-time type checking, prevents runtime panics")
	fmt.Println("2. Supports safe conversion between different types")
	fmt.Println("3. Zero reflection overhead, nanosecond performance")
	fmt.Println("4. Clean function composition, no builder needed")
	fmt.Println("5. Complete type safety, Chain3 ensures type matching")

	// Demonstrate error handling and retry
	fmt.Println("\n=== Error Handling and Retry Example ===")

	robustPipeline := pipeline.NewDynamicChain[string]().
		Then(pipeline.WithRetry(
			func(ctx context.Context, s string) (string, error) {
				if s == "fail" {
					return "", fmt.Errorf("intentional failure")
				}
				i, err := strconv.Atoi(s)
				if err != nil {
					return "", err
				}
				return strconv.Itoa(i), nil
			},
			3,                    // Retry 3 times
			100*time.Millisecond, // Retry interval
		)).
		Then(func(ctx context.Context, s string) (string, error) {
			i, _ := strconv.Atoi(s)
			return fmt.Sprintf("robust result: %d", i*10), nil
		}).
		ToFunc()

	testInputs := []string{"123", "fail", "456"}
	for _, input := range testInputs {
		fmt.Printf("\nProcessing input: %s\n", input)
		result, err := robustPipeline(ctx, input)
		if err != nil {
			fmt.Printf("  Final error: %v\n", err)
		} else {
			fmt.Printf("  Success result: %s\n", result)
		}
	}
}
