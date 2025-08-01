package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

func TestFixedDelayRetry(t *testing.T) {
	tests := []struct {
		name        string
		maxAttempts int
		delay       time.Duration
		attempt     int
		wantDelay   time.Duration
	}{
		{
			name:        "first attempt",
			maxAttempts: 3,
			delay:       100 * time.Millisecond,
			attempt:     1,
			wantDelay:   100 * time.Millisecond,
		},
		{
			name:        "second attempt",
			maxAttempts: 3,
			delay:       100 * time.Millisecond,
			attempt:     2,
			wantDelay:   100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewFixedDelayRetry(tt.maxAttempts, tt.delay)

			delay := policy.NextDelay(tt.attempt)
			if delay != tt.wantDelay {
				t.Errorf("NextDelay() = %v, want %v", delay, tt.wantDelay)
			}
		})
	}
}

func TestExponentialBackoffRetry(t *testing.T) {
	tests := []struct {
		name         string
		maxAttempts  int
		initialDelay time.Duration
		multiplier   float64
		attempt      int
		wantDelay    time.Duration
	}{
		{
			name:         "first attempt",
			maxAttempts:  3,
			initialDelay: 100 * time.Millisecond,
			multiplier:   2.0,
			attempt:      1,
			wantDelay:    100 * time.Millisecond,
		},
		{
			name:         "second attempt",
			maxAttempts:  3,
			initialDelay: 100 * time.Millisecond,
			multiplier:   2.0,
			attempt:      2,
			wantDelay:    200 * time.Millisecond,
		},
		{
			name:         "third attempt",
			maxAttempts:  3,
			initialDelay: 100 * time.Millisecond,
			multiplier:   2.0,
			attempt:      3,
			wantDelay:    400 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewExponentialBackoffRetry(tt.maxAttempts, tt.initialDelay,
				WithMultiplier(tt.multiplier))

			delay := policy.NextDelay(tt.attempt)
			if delay != tt.wantDelay {
				t.Errorf("NextDelay() = %v, want %v", delay, tt.wantDelay)
			}
		})
	}
}

func TestLinearBackoffRetry(t *testing.T) {
	tests := []struct {
		name         string
		maxAttempts  int
		initialDelay time.Duration
		increment    time.Duration
		attempt      int
		wantDelay    time.Duration
	}{
		{
			name:         "first attempt",
			maxAttempts:  3,
			initialDelay: 100 * time.Millisecond,
			increment:    50 * time.Millisecond,
			attempt:      1,
			wantDelay:    100 * time.Millisecond,
		},
		{
			name:         "second attempt",
			maxAttempts:  3,
			initialDelay: 100 * time.Millisecond,
			increment:    50 * time.Millisecond,
			attempt:      2,
			wantDelay:    150 * time.Millisecond,
		},
		{
			name:         "third attempt",
			maxAttempts:  3,
			initialDelay: 100 * time.Millisecond,
			increment:    50 * time.Millisecond,
			attempt:      3,
			wantDelay:    200 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewLinearBackoffRetry(tt.maxAttempts, tt.initialDelay, tt.increment)

			delay := policy.NextDelay(tt.attempt)
			if delay != tt.wantDelay {
				t.Errorf("NextDelay() = %v, want %v", delay, tt.wantDelay)
			}
		})
	}
}

func TestCustomRetry(t *testing.T) {
	customDelayFunc := func(attempt int) time.Duration {
		return time.Duration(attempt*attempt) * 100 * time.Millisecond
	}

	policy := NewCustomRetry(3, customDelayFunc)

	tests := []struct {
		attempt   int
		wantDelay time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 900 * time.Millisecond},
	}

	for _, tt := range tests {
		delay := policy.NextDelay(tt.attempt)
		if delay != tt.wantDelay {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, delay, tt.wantDelay)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	policy := NewFixedDelayRetry(3, 100*time.Millisecond)

	tests := []struct {
		name    string
		err     error
		attempt int
		want    bool
	}{
		{
			name:    "retryable error, first attempt",
			err:     types.ErrTimeout,
			attempt: 1,
			want:    true,
		},
		{
			name:    "retryable error, max attempts reached",
			err:     types.ErrTimeout,
			attempt: 3,
			want:    false,
		},
		{
			name:    "non-retryable error",
			err:     types.ErrPipelineClosed,
			attempt: 1,
			want:    false,
		},
		{
			name:    "nil error",
			err:     nil,
			attempt: 1,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.ShouldRetry(tt.err, tt.attempt)
			if got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryConditions(t *testing.T) {
	tests := []struct {
		name      string
		condition RetryCondition
		err       error
		want      bool
	}{
		{
			name:      "default condition with retryable error",
			condition: DefaultRetryCondition,
			err:       types.ErrTimeout,
			want:      true,
		},
		{
			name:      "default condition with non-retryable error",
			condition: DefaultRetryCondition,
			err:       types.ErrPipelineClosed,
			want:      false,
		},
		{
			name:      "retryable error types with timeout",
			condition: RetryableErrorTypes,
			err:       types.ErrTimeout,
			want:      true,
		},
		{
			name:      "retryable error types with worker pool full",
			condition: RetryableErrorTypes,
			err:       types.ErrWorkerPoolFull,
			want:      true,
		},
		{
			name:      "retryable error types with closed pipeline",
			condition: RetryableErrorTypes,
			err:       types.ErrPipelineClosed,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.condition(tt.err)
			if got != tt.want {
				t.Errorf("condition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyWithJitter(t *testing.T) {
	policy := NewFixedDelayRetry(3, 100*time.Millisecond,
		WithJitter(true, 0.1))

	// Due to jitter randomness, we can only test if delay is within reasonable range
	baseDelay := 100 * time.Millisecond
	delay := policy.NextDelay(1)

	// Jitter should be within ±10% range
	minDelay := time.Duration(float64(baseDelay) * 0.85) // Consider some error margin
	maxDelay := time.Duration(float64(baseDelay) * 1.15)

	if delay < minDelay || delay > maxDelay {
		t.Errorf("Jittered delay %v not in expected range [%v, %v]", delay, minDelay, maxDelay)
	}
}

func TestRetryPolicyReset(t *testing.T) {
	policy := NewFixedDelayRetry(3, 100*time.Millisecond)

	// Should be no difference before and after reset (fixed delay policy is stateless)
	delay1 := policy.NextDelay(1)
	policy.Reset()
	delay2 := policy.NextDelay(1)

	if delay1 != delay2 {
		t.Errorf("Delay after reset %v != delay before reset %v", delay2, delay1)
	}
}

func TestMaxDelayLimit(t *testing.T) {
	maxDelay := 500 * time.Millisecond
	policy := NewExponentialBackoffRetry(10, 100*time.Millisecond,
		WithMaxDelay(maxDelay))

	// Test if high attempt counts are limited by maximum delay
	delay := policy.NextDelay(10) // This should exceed max delay

	if delay > maxDelay {
		t.Errorf("Delay %v exceeds max delay %v", delay, maxDelay)
	}
}

func TestRetryableErrorWrapper(t *testing.T) {
	baseErr := errors.New("base error")
	retryableErr := &types.RetryableError{
		Err:        baseErr,
		Retryable:  true,
		RetryAfter: 200 * time.Millisecond,
	}

	// Test if RetryableError is correctly identified
	if !types.IsRetryable(retryableErr) {
		t.Error("RetryableError should be identified as retryable")
	}

	delay := types.GetRetryDelay(retryableErr)
	if delay != 200*time.Millisecond {
		t.Errorf("GetRetryDelay() = %v, want %v", delay, 200*time.Millisecond)
	}

	// Test default retry condition
	if !DefaultRetryCondition(retryableErr) {
		t.Error("DefaultRetryCondition should return true for RetryableError")
	}
}

func TestPipelineErrorRetry(t *testing.T) {
	baseErr := types.ErrTimeout
	pipelineErr := types.NewPipelineError("test-operation", "input", baseErr)

	// Pipeline error should determine retryability based on its underlying error
	condition := DefaultRetryCondition(pipelineErr)
	expected := RetryableErrorTypes(baseErr)

	if condition != expected {
		t.Errorf("DefaultRetryCondition for PipelineError = %v, want %v", condition, expected)
	}
}

// Benchmark tests
func BenchmarkFixedDelayRetry(b *testing.B) {
	policy := NewFixedDelayRetry(3, 100*time.Millisecond)
	err := types.ErrTimeout

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.ShouldRetry(err, 1)
		policy.NextDelay(1)
	}
}

func BenchmarkExponentialBackoffRetry(b *testing.B) {
	policy := NewExponentialBackoffRetry(3, 100*time.Millisecond)
	err := types.ErrTimeout

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.ShouldRetry(err, 1)
		policy.NextDelay(1)
	}
}

func BenchmarkRetryCondition(b *testing.B) {
	err := types.ErrTimeout

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DefaultRetryCondition(err)
	}
}
