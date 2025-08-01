// Package retry provides complete retry mechanism implementation with support for multiple retry policies and seamless Pipeline integration.
//
// Key Features:
//
// 1. Multiple retry policies:
//   - FixedDelayRetry: Fixed delay retry
//   - ExponentialBackoffRetry: Exponential backoff retry
//   - LinearBackoffRetry: Linear backoff retry
//   - CustomRetry: Custom retry policy
//
// 2. Advanced backoff algorithms:
//   - FixedBackoff: Fixed delay
//   - ExponentialBackoff: Exponential backoff
//   - LinearBackoff: Linear backoff
//   - FibonacciBackoff: Fibonacci backoff
//   - DecorrelatedJitterBackoff: Decorrelated jitter backoff
//
// 3. Jitter support:
//   - FullJitter: Full jitter
//   - EqualJitter: Equal jitter
//   - ExponentialJitter: Exponential jitter
//
// 4. Retry executor:
//   - Supports synchronous and asynchronous execution
//   - Context cancellation and timeout support
//   - Retry statistics and metrics collection
//   - Event notification mechanism
//
// 5. Pipeline integration:
//   - Global Pipeline retry
//   - Stage-level retry
//   - Configurable retry conditions
//   - Retry metrics collection
//
// Basic usage example:
//
//	// Create retry policy
//	policy := retry.NewExponentialBackoffRetry(3, 100*time.Millisecond)
//
//	// Create retry executor
//	executor := retry.NewRetryExecutor(policy)
//
//	// Execute function with retry
//	result, err := retry.Execute(executor, ctx, func(ctx context.Context) (string, error) {
//		// Your business logic
//		return doSomething()
//	})
//
// Pipeline integration example:
//
//	// Create retry configuration
//	config := retry.NewRetryConfigBuilder().
//		WithGlobalPolicy(retry.NewFixedDelayRetry(3, 100*time.Millisecond)).
//		WithStageRetry("critical-stage", retry.NewExponentialBackoffRetry(5, 200*time.Millisecond)).
//		WithMetrics(true).
//		Build()
//
//	// Create retryable Pipeline
//	retryablePipeline := retry.NewRetryablePipeline(originalPipeline, config)
//
// Custom retry conditions:
//
//	customCondition := func(err error) bool {
//		// Custom retry logic
//		return isTemporaryError(err)
//	}
//
//	policy := retry.NewFixedDelayRetry(3, 100*time.Millisecond,
//		retry.WithRetryCondition(customCondition))
//
// Jitter configuration:
//
//	policy := retry.NewExponentialBackoffRetry(3, 100*time.Millisecond,
//		retry.WithMultiplier(1.5),
//		retry.WithMaxDelay(10*time.Second))
//
//	// Enable jitter
//	policy = retry.NewFixedDelayRetry(3, 100*time.Millisecond,
//		retry.WithJitter(true, 0.1)) // 10% jitter
//
// Event handling:
//
//	handler := retry.NewDefaultEventHandler(logger)
//	executor := retry.NewRetryExecutor(policy,
//		retry.WithEventHandler(handler))
//
// Metrics collection:
//
//	collector := &MyMetricsCollector{}
//	executor := retry.NewRetryExecutor(policy,
//		retry.WithMetricsCollector(collector))
//
// Performance considerations:
//
// 1. Retry policies are lightweight and suitable for high-frequency use
// 2. Exponential backoff includes maximum delay limits to prevent excessive waiting
// 3. Jitter mechanism avoids thundering herd problems
// 4. Statistics collection has minimal performance impact
// 5. Supports context cancellation to avoid resource leaks
//
// Error handling:
//
// The retry mechanism integrates seamlessly with existing error handling systems:
// - Supports RetryableError types
// - Automatically identifies retryable error types
// - Preserves complete error context on retry failure
//
// Thread safety:
//
// All public types and methods are thread-safe and can be safely used in concurrent environments.
package retry
