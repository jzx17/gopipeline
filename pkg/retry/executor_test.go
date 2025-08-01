package retry

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

func TestRetryExecutor_Execute_Success(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	result, err := Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	stats := executor.GetStats()
	if stats.TotalAttempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", stats.TotalAttempts)
	}
	if stats.TotalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", stats.TotalSuccesses)
	}
	if stats.TotalRetries != 0 {
		t.Errorf("Expected 0 retries, got %d", stats.TotalRetries)
	}
}

func TestRetryExecutor_Execute_RetrySuccess(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	var attempts int32
	result, err := Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			return "", types.ErrTimeout // Retryable error
		}
		return "success", nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	stats := executor.GetStats()
	if stats.TotalAttempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", stats.TotalAttempts)
	}
	if stats.TotalRetries != 1 {
		t.Errorf("Expected 1 retry operation, got %d", stats.TotalRetries)
	}
}

func TestRetryExecutor_Execute_MaxAttemptsReached(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	var attempts int32
	result, err := Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		atomic.AddInt32(&attempts, 1)
		return "", types.ErrTimeout // Retryable error
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty result, got %v", result)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	stats := executor.GetStats()
	if stats.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.TotalFailures)
	}
}

func TestRetryExecutor_Execute_NonRetryableError(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	var attempts int32
	result, err := Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		atomic.AddInt32(&attempts, 1)
		return "", types.ErrPipelineClosed // Non-retryable error
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty result, got %v", result)
	}

	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}

	stats := executor.GetStats()
	if stats.TotalRetries != 0 {
		t.Errorf("Expected 0 retries, got %d", stats.TotalRetries)
	}
}

func TestRetryExecutor_Execute_ContextCanceled(t *testing.T) {
	policy := NewFixedDelayRetry(3, 100*time.Millisecond)
	executor := NewRetryExecutor(policy)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context during first retry delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	var attempts int32
	result, err := Execute(executor, ctx, func(ctx context.Context) (string, error) {
		atomic.AddInt32(&attempts, 1)
		return "", types.ErrTimeout
	})

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty result, got %v", result)
	}

	// Should have at least one attempt
	if atomic.LoadInt32(&attempts) < 1 {
		t.Errorf("Expected at least 1 attempt, got %d", attempts)
	}
}

func TestRetryExecutor_Execute_ContextTimeout(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := Execute(executor, ctx, func(ctx context.Context) (string, error) {
		// Simulate long-running operation
		time.Sleep(30 * time.Millisecond)
		return "", types.ErrTimeout
	})

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty result, got %v", result)
	}
}

func TestRetryExecutor_ExecuteAsync(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	var attempts int32
	resultChan := ExecuteAsync(executor, context.Background(), func(ctx context.Context) (string, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 2 {
			return "", types.ErrTimeout
		}
		return "async success", nil
	})

	select {
	case result := <-resultChan:
		if result.Error != nil {
			t.Fatalf("Expected no error, got %v", result.Error)
		}
		if result.Value != "async success" {
			t.Errorf("Expected 'async success', got %v", result.Value)
		}
		if result.Duration <= 0 {
			t.Error("Expected positive duration")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for async result")
	}

	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryExecutor_WithEventHandler(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)

	var events []string
	handler := &testEventHandler{events: &events}
	executor := NewRetryExecutor(policy, WithEventHandler(handler))

	var attempts int32
	_, err := Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			return "", types.ErrTimeout
		}
		return "success", nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Expect retry attempt and success events (specific order may contain multiple attempts)
	hasRetryAttempt := false
	hasRetrySuccess := false

	for _, event := range events {
		switch event {
		case "retry_attempt":
			hasRetryAttempt = true
		case "retry_success":
			hasRetrySuccess = true
		}
	}

	if !hasRetryAttempt {
		t.Error("Expected retry_attempt event")
	}
	if !hasRetrySuccess {
		t.Error("Expected retry_success event")
	}
}

func TestRetryExecutor_GetStats(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	// Execute successful operation
	var attempts1 int32
	Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		attempt := atomic.AddInt32(&attempts1, 1)
		if attempt < 2 {
			return "", types.ErrTimeout
		}
		return "success", nil
	})

	// Execute failed operation
	var attempts2 int32
	Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		atomic.AddInt32(&attempts2, 1)
		return "", types.ErrTimeout
	})

	stats := executor.GetStats()

	if stats.TotalAttempts != 5 { // 2 + 3 attempts
		t.Errorf("Expected 5 total attempts, got %d", stats.TotalAttempts)
	}
	if stats.TotalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", stats.TotalSuccesses)
	}
	if stats.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.TotalFailures)
	}
	if stats.TotalRetries != 2 { // 1 successful retry + 1 failed operation
		t.Errorf("Expected 2 retry operations, got %d", stats.TotalRetries)
	}
	if stats.AverageAttempts != 2.5 { // (2 + 3) / 2
		t.Errorf("Expected 2.5 average attempts, got %f", stats.AverageAttempts)
	}
}

func TestRetryExecutor_ResetStats(t *testing.T) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	// Execute some operations
	Execute(executor, context.Background(), func(ctx context.Context) (string, error) {
		return "success", nil
	})

	// Reset statistics
	executor.ResetStats()

	stats := executor.GetStats()
	if stats.TotalAttempts != 0 {
		t.Errorf("Expected 0 attempts after reset, got %d", stats.TotalAttempts)
	}
	if stats.TotalSuccesses != 0 {
		t.Errorf("Expected 0 successes after reset, got %d", stats.TotalSuccesses)
	}
}

// Test helper types
type testEventHandler struct {
	events *[]string
}

func (h *testEventHandler) OnRetryAttempt(ctx context.Context, attempt int, err error) {
	*h.events = append(*h.events, "retry_attempt")
}

func (h *testEventHandler) OnRetrySuccess(ctx context.Context, attempt int, duration time.Duration) {
	*h.events = append(*h.events, "retry_success")
}

func (h *testEventHandler) OnRetryFailure(ctx context.Context, attempt int, err error) {
	*h.events = append(*h.events, "retry_failure")
}

func (h *testEventHandler) OnMaxAttemptsReached(ctx context.Context, attempt int, err error) {
	*h.events = append(*h.events, "max_attempts_reached")
}

// Benchmark tests
func BenchmarkRetryExecutor_NoRetry(b *testing.B) {
	policy := NewFixedDelayRetry(3, 10*time.Millisecond)
	executor := NewRetryExecutor(policy)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Execute(executor, context.Background(), func(ctx context.Context) (int, error) {
			return i, nil
		})
	}
}

func BenchmarkRetryExecutor_WithRetry(b *testing.B) {
	policy := NewFixedDelayRetry(3, 1*time.Millisecond) // Reduce delay to speed up test
	executor := NewRetryExecutor(policy)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var attempts int32
		Execute(executor, context.Background(), func(ctx context.Context) (int, error) {
			attempt := atomic.AddInt32(&attempts, 1)
			if attempt < 2 {
				return 0, types.ErrTimeout
			}
			return i, nil
		})
	}
}

func BenchmarkRetryExecutor_Async(b *testing.B) {
	policy := NewFixedDelayRetry(3, 1*time.Millisecond)
	executor := NewRetryExecutor(policy)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resultChan := ExecuteAsync(executor, context.Background(), func(ctx context.Context) (int, error) {
			return i, nil
		})
		<-resultChan
	}
}
