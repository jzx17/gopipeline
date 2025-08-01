// Package main demonstrates retry mechanism usage
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/jzx17/gopipeline/pkg/retry"
)

func main() {
	fmt.Println("=== Retry Mechanism Example ===")

	// Example 1: Basic retry policy
	fmt.Println("\n1. Basic Retry Policy")
	basicRetryExample()

	// Example 2: Exponential backoff policy
	fmt.Println("\n2. Exponential Backoff Policy")
	exponentialBackoffExample()

	// Example 3: Linear backoff policy
	fmt.Println("\n3. Linear Backoff Policy")
	linearBackoffExample()

	// Example 4: Fixed delay policy
	fmt.Println("\n4. Fixed Delay Policy")
	fixedDelayExample()

	// Example 5: Retry executor
	fmt.Println("\n5. Retry Executor")
	executorExample()
}

func basicRetryExample() {
	// Create basic retry policy
	policy := retry.NewBaseRetryPolicy(3)

	attemptCount := 0
	operation := func() error {
		attemptCount++
		fmt.Printf("  Attempt #%d\n", attemptCount)

		if attemptCount < 3 {
			return errors.New("temporary failure")
		}
		return nil // Third attempt succeeds
	}

	// Manual retry logic
	ctx := context.Background()
	for attempt := 0; attempt < policy.MaxAttempts(); attempt++ {
		err := operation()
		if err == nil {
			fmt.Printf("  Operation successful after %d attempts\n", attempt+1)
			break
		}

		if !policy.ShouldRetry(err, attempt) {
			fmt.Printf("  Retry policy decided not to retry: %v\n", err)
			break
		}

		if attempt < policy.MaxAttempts()-1 {
			delay := 100 * time.Millisecond // Fixed delay
			fmt.Printf("  Waiting %v before retry...\n", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}
	}
}

func exponentialBackoffExample() {
	// Create exponential backoff policy
	policy := retry.NewExponentialBackoffRetry(4, 50*time.Millisecond)

	fmt.Printf("Exponential backoff policy, max attempts: %d\n", policy.MaxAttempts())
}

func linearBackoffExample() {
	// Create linear backoff policy
	policy := retry.NewLinearBackoffRetry(5, 100*time.Millisecond, 100*time.Millisecond)

	fmt.Printf("Linear backoff policy, max attempts: %d\n", policy.MaxAttempts())
}

func fixedDelayExample() {
	// Create fixed delay policy
	policy := retry.NewFixedDelayRetry(3, 200*time.Millisecond)

	fmt.Printf("Fixed delay policy, max attempts: %d\n", policy.MaxAttempts())
}

func executorExample() {
	// Create retry executor
	policy := retry.NewExponentialBackoffRetry(3, 100*time.Millisecond)
	executor := retry.NewRetryExecutor(policy)

	// Simulate unstable operation
	attemptCount := 0
	operation := func(ctx context.Context) (string, error) {
		attemptCount++
		fmt.Printf("  Executor attempt #%d\n", attemptCount)

		// Random failure
		if rand.Float32() < 0.6 && attemptCount < 3 {
			return "", errors.New("random failure")
		}
		return "success", nil
	}

	// Use executor for retry
	ctx := context.Background()
	result, err := retry.Execute(executor, ctx, operation)
	if err != nil {
		fmt.Printf("  Executor final failure: %v\n", err)
	} else {
		fmt.Printf("  Executor success: %s, total attempts: %d\n", result, attemptCount)
	}
}

// Network operation retry example
func networkOperationExample() {
	fmt.Println("\n=== Network Operation Retry Example ===")

	policy := retry.NewExponentialBackoffRetry(5, 200*time.Millisecond)
	executor := retry.NewRetryExecutor(policy)

	// Simulate network request
	requestCount := 0
	networkRequest := func(ctx context.Context) (string, error) {
		requestCount++
		fmt.Printf("Sending network request #%d\n", requestCount)

		// Simulate network errors for first few attempts
		switch requestCount {
		case 1:
			return "", errors.New("connection timeout")
		case 2:
			return "", errors.New("DNS resolution failed")
		case 3:
			return "", errors.New("connection refused")
		default:
			fmt.Println("Request successful!")
			return "response data", nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	result, err := retry.Execute(executor, ctx, networkRequest)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Network request final failure: %v (duration: %v)\n", err, duration)
	} else {
		fmt.Printf("Network request success: %s (duration: %v, attempts: %d)\n", result, duration, requestCount)
	}
}
