package retry

import (
	"fmt"
	"testing"
	"time"
)

func TestFixedBackoff(t *testing.T) {
	delay := 100 * time.Millisecond
	backoff := NewFixedBackoff(delay)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, delay},
		{2, delay},
		{3, delay},
		{10, delay},
	}

	for _, tt := range tests {
		got := backoff.NextDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestExponentialBackoff(t *testing.T) {
	initialDelay := 100 * time.Millisecond
	backoff := NewExponentialBackoff(initialDelay,
		WithBackoffMultiplier(2.0),
		WithBackoffMaxDelay(1*time.Second))

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1000 * time.Millisecond},  // Limited by max delay
		{10, 1000 * time.Millisecond}, // Limited by max delay
	}

	for _, tt := range tests {
		got := backoff.NextDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestLinearBackoff(t *testing.T) {
	initialDelay := 100 * time.Millisecond
	increment := 50 * time.Millisecond
	backoff := NewLinearBackoff(initialDelay, increment,
		WithBackoffMaxDelay(500*time.Millisecond))

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 150 * time.Millisecond},
		{3, 200 * time.Millisecond},
		{4, 250 * time.Millisecond},
		{5, 300 * time.Millisecond},
		{10, 500 * time.Millisecond}, // Limited by max delay
	}

	for _, tt := range tests {
		got := backoff.NextDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestFibonacciBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	backoff := NewFibonacciBackoff(baseDelay,
		WithBackoffMaxDelay(2*time.Second))

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},  // fib(0) = 1
		{2, 100 * time.Millisecond},  // fib(1) = 1
		{3, 200 * time.Millisecond},  // fib(2) = 2
		{4, 300 * time.Millisecond},  // fib(3) = 3
		{5, 500 * time.Millisecond},  // fib(4) = 5
		{6, 800 * time.Millisecond},  // fib(5) = 8
		{7, 1300 * time.Millisecond}, // fib(6) = 13
		{8, 2000 * time.Millisecond}, // fib(7) = 21, but limited by maxDelay
	}

	for _, tt := range tests {
		got := backoff.NextDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestFibonacciBackoff_CacheOptimization(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	backoff := NewFibonacciBackoff(baseDelay)

	// Test if cache works properly
	delay1 := backoff.NextDelay(5)
	delay2 := backoff.NextDelay(5)

	if delay1 != delay2 {
		t.Errorf("Cached fibonacci calculation failed: %v != %v", delay1, delay2)
	}

	// Test if cache is cleared after reset
	backoff.Reset()
	delay3 := backoff.NextDelay(5)

	if delay3 != delay1 {
		t.Errorf("Reset didn't work correctly: %v != %v", delay3, delay1)
	}
}

func TestDecorrelatedJitterBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	capDelay := 1 * time.Second
	backoff := NewDecorrelatedJitterBackoff(baseDelay, capDelay)

	// Test results of multiple calls
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = backoff.NextDelay(i + 1)
	}

	// Verify delays are within reasonable range
	for i, delay := range delays {
		if delay < baseDelay {
			t.Errorf("Delay %d (%v) is less than base delay (%v)", i, delay, baseDelay)
		}
		if delay > capDelay {
			t.Errorf("Delay %d (%v) exceeds cap delay (%v)", i, delay, capDelay)
		}
	}

	// Verify delays have variation (decorrelation)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("All delays are the same, jitter is not working")
	}
}

func TestDecorrelatedJitterBackoff_Reset(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	capDelay := 1 * time.Second
	backoff := NewDecorrelatedJitterBackoff(baseDelay, capDelay)

	// Execute several times to change internal state
	for i := 0; i < 5; i++ {
		backoff.NextDelay(i + 1)
	}

	// Reset
	backoff.Reset()

	// Should work normally after reset (decorrelated jitter won't return exact baseDelay)
	delay := backoff.NextDelay(1)
	if delay < baseDelay || delay > capDelay {
		t.Errorf("After reset, delay should be in range [%v, %v], got %v", baseDelay, capDelay, delay)
	}
}

func TestJitterFunctions(t *testing.T) {
	delay := 1000 * time.Millisecond

	// Test FullJitter
	for i := 0; i < 100; i++ {
		jittered := FullJitter(delay)
		if jittered < 0 || jittered > delay {
			t.Errorf("FullJitter result %v out of range [0, %v]", jittered, delay)
		}
	}

	// Test EqualJitter
	half := delay / 2
	for i := 0; i < 100; i++ {
		jittered := EqualJitter(delay)
		if jittered < half || jittered > delay {
			t.Errorf("EqualJitter result %v out of range [%v, %v]", jittered, half, delay)
		}
	}

	// Test ExponentialJitter
	expJitter := ExponentialJitter(0.1)
	for i := 0; i < 100; i++ {
		jittered := expJitter(delay)
		if jittered < 0 {
			t.Errorf("ExponentialJitter result %v should not be negative", jittered)
		}
		// Due to exponential distribution characteristics, we only check non-negativity
	}
}

func TestJitterWithZeroDelay(t *testing.T) {
	zeroDelay := time.Duration(0)

	if FullJitter(zeroDelay) != 0 {
		t.Error("FullJitter with zero delay should return 0")
	}

	if EqualJitter(zeroDelay) != 0 {
		t.Error("EqualJitter with zero delay should return 0")
	}

	expJitter := ExponentialJitter(0.1)
	if expJitter(zeroDelay) != 0 {
		t.Error("ExponentialJitter with zero delay should return 0")
	}
}

func TestBackoffStrategyOptions(t *testing.T) {
	t.Run("FixedBackoff with jitter", func(t *testing.T) {
		delay := 100 * time.Millisecond
		backoff := NewFixedBackoff(delay, WithBackoffJitter(EqualJitter))

		// Due to jitter randomness, test multiple times to ensure variation
		results := make(map[time.Duration]bool)
		for i := 0; i < 50; i++ {
			result := backoff.NextDelay(1)
			results[result] = true

			// Ensure results are within reasonable range
			if result < delay/2 || result > delay {
				t.Errorf("Jittered delay %v out of expected range [%v, %v]", result, delay/2, delay)
			}
		}

		// Should have multiple different results
		if len(results) < 2 {
			t.Error("Jitter should produce varying results")
		}
	})

	t.Run("ExponentialBackoff with custom multiplier", func(t *testing.T) {
		initialDelay := 100 * time.Millisecond
		multiplier := 1.5
		backoff := NewExponentialBackoff(initialDelay,
			WithBackoffMultiplier(multiplier))

		expected := time.Duration(float64(initialDelay) * multiplier)
		got := backoff.NextDelay(2)

		if got != expected {
			t.Errorf("Custom multiplier: expected %v, got %v", expected, got)
		}
	})
}

func TestBackoffEdgeCases(t *testing.T) {
	t.Run("zero and negative attempts", func(t *testing.T) {
		backoff := NewExponentialBackoff(100 * time.Millisecond)

		// Zero and negative attempts should be handled as first attempt
		delay0 := backoff.NextDelay(0)
		delay1 := backoff.NextDelay(1)
		delayNeg := backoff.NextDelay(-1)

		if delay0 != delay1 || delay1 != delayNeg {
			t.Errorf("Zero/negative attempts handling: %v, %v, %v", delay0, delay1, delayNeg)
		}
	})

	t.Run("very large attempts", func(t *testing.T) {
		maxDelay := 10 * time.Second
		backoff := NewExponentialBackoff(1*time.Millisecond,
			WithBackoffMaxDelay(maxDelay))

		// Very large attempt counts should be limited by max delay
		delay := backoff.NextDelay(100)
		if delay > maxDelay {
			t.Errorf("Large attempt delay %v exceeds max %v", delay, maxDelay)
		}
	})
}

func TestBackoffReset(t *testing.T) {
	strategies := []BackoffStrategy{
		NewFixedBackoff(100 * time.Millisecond),
		NewExponentialBackoff(100 * time.Millisecond),
		NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond),
		NewFibonacciBackoff(100 * time.Millisecond),
		NewDecorrelatedJitterBackoff(100*time.Millisecond, 1*time.Second),
	}

	for i, strategy := range strategies {
		t.Run(fmt.Sprintf("strategy_%d", i), func(t *testing.T) {
			// Execute some operations to change state
			for j := 1; j <= 5; j++ {
				strategy.NextDelay(j)
			}

			// Reset
			strategy.Reset()

			// Should work normally after reset
			delay := strategy.NextDelay(1)
			if delay <= 0 {
				t.Errorf("After reset, NextDelay should return positive duration, got %v", delay)
			}
		})
	}
}

// Benchmark tests
func BenchmarkFixedBackoff(b *testing.B) {
	backoff := NewFixedBackoff(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.NextDelay(i % 10)
	}
}

func BenchmarkExponentialBackoff(b *testing.B) {
	backoff := NewExponentialBackoff(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.NextDelay(i % 10)
	}
}

func BenchmarkLinearBackoff(b *testing.B) {
	backoff := NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.NextDelay(i % 10)
	}
}

func BenchmarkFibonacciBackoff(b *testing.B) {
	backoff := NewFibonacciBackoff(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.NextDelay(i % 10)
	}
}

func BenchmarkDecorrelatedJitterBackoff(b *testing.B) {
	backoff := NewDecorrelatedJitterBackoff(100*time.Millisecond, 1*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.NextDelay(i % 10)
	}
}

func BenchmarkJitterFunctions(b *testing.B) {
	delay := 1000 * time.Millisecond

	b.Run("FullJitter", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			FullJitter(delay)
		}
	})

	b.Run("EqualJitter", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			EqualJitter(delay)
		}
	})

	b.Run("ExponentialJitter", func(b *testing.B) {
		jitter := ExponentialJitter(0.1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			jitter(delay)
		}
	})
}
