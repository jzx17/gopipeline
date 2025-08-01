// Package main demonstrates functional Pipeline usage
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jzx17/gopipeline/pkg/pipeline"
)

func main() {
	fmt.Println("=== Functional Pipeline Example ===")

	// Example 1: Simple chain processing
	fmt.Println("\n1. Simple Chain Processing")
	simpleChainExample()

	// Example 2: Complex type conversion chain
	fmt.Println("\n2. Complex Type Conversion Chain")
	complexChainExample()

	// Example 3: Conditional processing
	fmt.Println("\n3. Conditional Processing")
	conditionalExample()

	// Example 4: Parallel processing
	fmt.Println("\n4. Parallel Processing")
	parallelExample()

	// Example 5: Error handling and retry
	fmt.Println("\n5. Error Handling and Retry")
	errorHandlingExample()

	// Example 6: Comparison of different chain types
	fmt.Println("\n6. Comparison of Different Chain Types")
	chainComparisonExample()
}

func simpleChainExample() {
	// Use dynamic chain for string processing
	processString := pipeline.NewDynamicChain[string]().
		Then(pipeline.Transform(strings.TrimSpace)).
		Then(pipeline.Transform(strings.ToUpper)).
		Then(pipeline.Transform(func(s string) string {
			return "PROCESSED: " + s
		})).
		ToFunc()

	ctx := context.Background()
	result, err := processString(ctx, "   hello world   ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Input: '%s'\n", "   hello world   ")
	fmt.Printf("Functional result: '%s'\n", result)
}

func complexChainExample() {
	// Use ChainBuilder to build complex processing chain
	processChain := pipeline.NewChainBuilder[string]().
		AddFunc("parse", func(ctx context.Context, s string) (string, error) {
			// string -> int -> string conversion
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", err
			}
			return strconv.Itoa(i), nil
		}).
		AddFunc("calculate", func(ctx context.Context, s string) (string, error) {
			// int -> float64 -> string
			i, _ := strconv.Atoi(s)
			result := float64(i) * 1.5
			return fmt.Sprintf("Result: %.2f", result), nil
		}).
		Build()

	ctx := context.Background()
	result, err := processChain(ctx, "  100  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Complex conversion chain result: %s\n", result)
}

func conditionalExample() {
	// Conditional processing: choose different processing based on number size
	processWithCondition := pipeline.NewChainBuilder[string]().
		AddFunc("parse_and_condition", func(ctx context.Context, s string) (string, error) {
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", err
			}

			if i > 50 {
				return fmt.Sprintf("Large number: %d", i*2), nil
			} else {
				return fmt.Sprintf("Small number: %d", i/2), nil
			}
		}).
		Build()

	ctx := context.Background()

	result1, _ := processWithCondition(ctx, "100")
	fmt.Printf("Conditional processing (100): %s\n", result1)

	result2, _ := processWithCondition(ctx, "30")
	fmt.Printf("Conditional processing (30): %s\n", result2)
}

func parallelExample() {
	// Parallel processing: calculate square and cube simultaneously
	parallelProcess := pipeline.NewChainBuilder[string]().
		AddFunc("parse_and_parallel", func(ctx context.Context, s string) (string, error) {
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", err
			}

			// Simulate parallel calculation
			square := i * i
			cube := i * i * i

			return fmt.Sprintf("Square: %d, Cube: %d", square, cube), nil
		}).
		Build()

	ctx := context.Background()
	result, err := parallelProcess(ctx, "5")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Parallel processing result: %s\n", result)
}

func errorHandlingExample() {
	// Add retry and error handling
	robustProcess := pipeline.NewChainBuilder[string]().
		AddFunc("risky_with_retry", func(ctx context.Context, s string) (string, error) {
			// Simulate retry logic
			maxAttempts := 3
			for attempt := 0; attempt < maxAttempts; attempt++ {
				if s == "fail" && attempt < 2 {
					fmt.Printf("    Attempt %d: Simulated failure\n", attempt+1)
					if attempt < maxAttempts-1 {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					return "", fmt.Errorf("Processing failed: intentional failure")
				}

				if s == "fail" {
					return "", fmt.Errorf("Processing failed: intentional failure")
				}

				return fmt.Sprintf("Successfully processed, length: %d", len(s)), nil
			}
			return "", fmt.Errorf("Retries exhausted")
		}).
		Build()

	ctx := context.Background()

	// Successful case
	result1, err := robustProcess(ctx, "hello")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success: %s\n", result1)
	}

	// Failure case
	result2, err := robustProcess(ctx, "fail")
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
	} else {
		fmt.Printf("Unexpected success: %s\n", result2)
	}
}

func chainComparisonExample() {
	ctx := context.Background()
	input := "42"

	// 1. DynamicChain - Most flexible
	dynamicChain := pipeline.NewDynamicChain[string]().
		Then(pipeline.Transform(strings.TrimSpace)).
		Then(pipeline.Transform(func(s string) string { return "Dynamic: " + s }))

	result1, _ := dynamicChain.Execute(ctx, input)
	fmt.Printf("DynamicChain: %s\n", result1)

	// 2. ArrayChain - Highest performance
	arrayChain := pipeline.NewArrayChain(
		pipeline.Transform(strings.TrimSpace),
		pipeline.Transform(func(s string) string { return "Array: " + s }),
	)

	result2, _ := arrayChain.Execute(ctx, input)
	fmt.Printf("ArrayChain: %s\n", result2)

	// 3. ImmutableChain - Functional style
	immutableChain := pipeline.NewImmutableChain(
		pipeline.Transform(strings.TrimSpace),
		pipeline.Transform(func(s string) string { return "Immutable: " + s }),
	)

	result3, _ := immutableChain.Execute(ctx, input)
	fmt.Printf("ImmutableChain: %s\n", result3)

	// 4. ChainBuilder - Most clear
	builderChain := pipeline.NewChainBuilder[string]().
		AddFunc("trim", func(ctx context.Context, s string) (string, error) {
			return strings.TrimSpace(s), nil
		}).
		AddFunc("prefix", func(ctx context.Context, s string) (string, error) {
			return "Builder: " + s, nil
		}).
		Build()

	result4, _ := builderChain(ctx, input)
	fmt.Printf("ChainBuilder: %s\n", result4)
}
