package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTransform(t *testing.T) {
	// Test basic Transform function
	upperCase := Transform(strings.ToUpper)

	ctx := context.Background()
	result, err := upperCase(ctx, "hello")

	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if result != "HELLO" {
		t.Errorf("Expected 'HELLO', got '%s'", result)
	}
}

func TestTryTransform(t *testing.T) {
	// Test Transform that may fail
	parseInt := TryTransform(func(s string) (int, error) {
		return strconv.Atoi(s)
	})

	ctx := context.Background()

	// Successful case
	result, err := parseInt(ctx, "123")
	if err != nil {
		t.Fatalf("TryTransform failed: %v", err)
	}
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// Failure case
	_, err = parseInt(ctx, "abc")
	if err == nil {
		t.Error("Expected error for invalid input")
	}
}

func TestDynamicChainReplacement(t *testing.T) {
	// Test dynamic chain composition to replace original Chain
	chain := NewDynamicChain[string]().
		Then(Transform(strings.TrimSpace)).
		Then(Transform(strings.ToUpper))

	ctx := context.Background()
	result, err := chain.Execute(ctx, "  hello  ")

	if err != nil {
		t.Fatalf("DynamicChain failed: %v", err)
	}

	if result != "HELLO" {
		t.Errorf("Expected 'HELLO', got '%s'", result)
	}
}

func TestArrayChainReplacement(t *testing.T) {
	// Test array chain to replace original Chain3
	chain := NewArrayChain(
		Transform(strings.TrimSpace),
		Transform(strings.ToUpper),
		Transform(func(s string) string { return "RESULT: " + s }),
	)

	ctx := context.Background()
	result, err := chain.Execute(ctx, "  hello  ")

	if err != nil {
		t.Fatalf("ArrayChain failed: %v", err)
	}

	expected := "RESULT: HELLO"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestImmutableChainReplacement(t *testing.T) {
	// Test immutable chain to replace original Chain4
	chain := NewImmutableChain(
		Transform(strings.TrimSpace),
		Transform(strings.ToUpper),
		Transform(func(s string) string { return "STEP1: " + s }),
		Transform(func(s string) string { return "FINAL: " + s }),
	)

	ctx := context.Background()
	result, err := chain.Execute(ctx, "  hello  ")

	if err != nil {
		t.Fatalf("ImmutableChain failed: %v", err)
	}

	expected := "FINAL: STEP1: HELLO"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestChainBuilderWithComplexProcessing(t *testing.T) {
	// Test complex data processing chain
	builder := NewChainBuilder[string]().
		AddFunc("trim_and_parse", func(ctx context.Context, s string) (string, error) {
			trimmed := strings.TrimSpace(s)
			i, err := strconv.Atoi(trimmed)
			if err != nil {
				return "", err
			}
			return strconv.Itoa(i), nil
		}).
		AddFunc("double_and_format", func(ctx context.Context, s string) (string, error) {
			i, _ := strconv.Atoi(s)
			return fmt.Sprintf("Number: %d", i*2), nil
		})

	process := builder.Build()
	ctx := context.Background()
	result, err := process(ctx, "  42  ")

	if err != nil {
		t.Fatalf("ChainBuilder with complex processing failed: %v", err)
	}

	expected := "Number: 84"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestWithRetry(t *testing.T) {
	attempts := 0
	failingFunc := TryTransform(func(s string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary failure")
		}
		return "success", nil
	})

	retryFunc := WithRetry(failingFunc, 3, 10*time.Millisecond)

	ctx := context.Background()
	result, err := retryFunc(ctx, "test")

	if err != nil {
		t.Fatalf("WithRetry failed: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetryFailure(t *testing.T) {
	alwaysFail := TryTransform(func(s string) (string, error) {
		return "", errors.New("always fails")
	})

	retryFunc := WithRetry(alwaysFail, 2, 1*time.Millisecond)

	ctx := context.Background()
	_, err := retryFunc(ctx, "test")

	if err == nil {
		t.Error("Expected error after retries exhausted")
	}

	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Error message should mention attempts: %v", err)
	}
}

func TestHandleErrors(t *testing.T) {
	failingFunc := TryTransform(func(s string) (string, error) {
		return "", errors.New("original error")
	})

	handledFunc := HandleErrors(failingFunc, func(err error) error {
		return fmt.Errorf("handled: %w", err)
	})

	ctx := context.Background()
	_, err := handledFunc(ctx, "test")

	if err == nil {
		t.Error("Expected error")
	}

	if !strings.Contains(err.Error(), "handled: original error") {
		t.Errorf("Error should be wrapped: %v", err)
	}
}

func TestConditional(t *testing.T) {
	process := Conditional(
		func(i int) bool { return i > 10 },
		Transform(func(i int) string { return fmt.Sprintf("big: %d", i) }),
		Transform(func(i int) string { return fmt.Sprintf("small: %d", i) }),
	)

	ctx := context.Background()

	// Test large number
	result1, err := process(ctx, 15)
	if err != nil {
		t.Fatalf("Conditional failed: %v", err)
	}
	if result1 != "big: 15" {
		t.Errorf("Expected 'big: 15', got '%s'", result1)
	}

	// Test small number
	result2, err := process(ctx, 5)
	if err != nil {
		t.Fatalf("Conditional failed: %v", err)
	}
	if result2 != "small: 5" {
		t.Errorf("Expected 'small: 5', got '%s'", result2)
	}
}

func TestParallel(t *testing.T) {
	process := Parallel(
		Transform(func(i int) int { return i * i }),     // Square
		Transform(func(i int) int { return i * i * i }), // Cube
	)

	ctx := context.Background()
	result, err := process(ctx, 3)

	if err != nil {
		t.Fatalf("Parallel failed: %v", err)
	}

	if result.R1 != 9 || result.R2 != 27 {
		t.Errorf("Expected {9, 27}, got {%d, %d}", result.R1, result.R2)
	}
}

func TestParallelWithError(t *testing.T) {
	process := Parallel(
		Transform(func(i int) int { return i * i }),
		TryTransform(func(i int) (int, error) { return 0, errors.New("error") }),
	)

	ctx := context.Background()
	_, err := process(ctx, 3)

	if err == nil {
		t.Error("Expected error from parallel execution")
	}
}

func TestNewFunctional(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	t.Run("create functional pipeline", func(t *testing.T) {
		fp := NewFunctional("test-pipeline", processFunc)

		if fp.name != "test-pipeline" {
			t.Errorf("expected name 'test-pipeline', got '%s'", fp.name)
		}

		if fp.process == nil {
			t.Error("expected process function to be set")
		}

		if fp.config == nil {
			t.Error("expected config to be set")
		}

		// Verifydefault configuration
		if fp.config.Workers != 1 {
			t.Errorf("expected default workers 1, got %d", fp.config.Workers)
		}

		if fp.config.BufferSize != 100 {
			t.Errorf("expected default buffer size 100, got %d", fp.config.BufferSize)
		}
	})
}

func TestFunctionalPipeline_Execute(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-functional", nil
	}

	fp := NewFunctional("test", processFunc)

	result, err := fp.Execute(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "test-functional" {
		t.Errorf("expected 'test-functional', got '%s'", result)
	}
}

func TestFunctionalPipeline_WithTimeout(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return input + "-delayed", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	fp := NewFunctional("test", processFunc)
	fpWithTimeout := fp.WithTimeout(50 * time.Millisecond)

	t.Run("timeout applied", func(t *testing.T) {
		if fpWithTimeout.config.Timeout != 50*time.Millisecond {
			t.Errorf("expected timeout 50ms, got %v", fpWithTimeout.config.Timeout)
		}
	})

	t.Run("execution with timeout", func(t *testing.T) {
		_, err := fpWithTimeout.Execute(context.Background(), "test")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded error, got: %v", err)
		}
	})
}

func TestFilterFunc(t *testing.T) {
	t.Run("filter pass", func(t *testing.T) {
		filterFunc := FilterFunc(func(input int) bool {
			return input > 5
		})

		result, err := filterFunc(context.Background(), 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("filter reject", func(t *testing.T) {
		filterFunc := FilterFunc(func(input int) bool {
			return input > 5
		})

		_, err := filterFunc(context.Background(), 3)
		if err == nil {
			t.Error("expected error for filtered item")
		}

		if err.Error() != "filtered out" {
			t.Errorf("expected 'filtered out' error, got: %v", err)
		}
	})
}

func TestFilterWithResult(t *testing.T) {
	t.Run("filter with result success", func(t *testing.T) {
		filterFunc := FilterWithResult(func(input int) (bool, error) {
			if input < 0 {
				return false, errors.New("negative number")
			}
			return input > 5, nil
		})

		result, err := filterFunc(context.Background(), 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("filter with error", func(t *testing.T) {
		filterFunc := FilterWithResult(func(input int) (bool, error) {
			if input < 0 {
				return false, errors.New("negative number")
			}
			return input > 5, nil
		})

		_, err := filterFunc(context.Background(), -1)
		if err == nil {
			t.Error("expected error for negative input")
		}

		if err.Error() != "negative number" {
			t.Errorf("expected 'negative number' error, got: %v", err)
		}
	})

	t.Run("filter reject", func(t *testing.T) {
		filterFunc := FilterWithResult(func(input int) (bool, error) {
			return input > 5, nil
		})

		_, err := filterFunc(context.Background(), 3)
		if err == nil {
			t.Error("expected error for filtered item")
		}
	})
}

func TestTap(t *testing.T) {
	var observed string
	observer := func(value string) {
		observed = value
	}

	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	tappedFunc := Tap(processFunc, observer)

	result, err := tappedFunc(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "test-processed" {
		t.Errorf("expected 'test-processed', got '%s'", result)
	}

	if observed != "test-processed" {
		t.Errorf("expected observed value 'test-processed', got '%s'", observed)
	}
}

func TestFunctionalPipelineTimeout(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return input + "-delayed", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	fp := NewFunctional("test", processFunc).WithTimeout(50 * time.Millisecond)

	_, err := fp.Execute(context.Background(), "test")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded error, got: %v", err)
	}
}

func TestComplexProcessingChain(t *testing.T) {
	// Complex string processing chain: trim -> parse -> calculate -> format
	builder := NewChainBuilder[string]().
		AddFunc("trim_and_validate", func(ctx context.Context, s string) (string, error) {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				return "", errors.New("empty input")
			}
			return trimmed, nil
		}).
		AddFunc("parse_and_calculate", func(ctx context.Context, s string) (string, error) {
			i, err := strconv.Atoi(s)
			if err != nil {
				return "", err
			}
			result := float64(i) * 1.5
			return fmt.Sprintf("%.2f", result), nil
		})

	process := builder.Build()
	ctx := context.Background()
	result, err := process(ctx, "  100  ")

	if err != nil {
		t.Fatalf("Complex processing chain failed: %v", err)
	}

	if result != "150.00" {
		t.Errorf("Expected '150.00', got '%s'", result)
	}
}

// Benchmark tests
func BenchmarkTransform(b *testing.B) {
	upperCase := Transform(strings.ToUpper)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = upperCase(ctx, "hello world")
	}
}

func BenchmarkDynamicChain(b *testing.B) {
	chain := NewDynamicChain[string]().
		Then(Transform(strings.TrimSpace)).
		Then(Transform(strings.ToUpper)).
		Then(Transform(func(s string) string { return "RESULT: " + s }))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chain.Execute(ctx, "  hello world  ")
	}
}

func BenchmarkComplexProcessingChain(b *testing.B) {
	builder := NewChainBuilder[string]().
		AddFunc("parse", func(ctx context.Context, s string) (string, error) {
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", err
			}
			result := float64(i) * 1.5
			return fmt.Sprintf("%.2f", result), nil
		})
	process := builder.Build()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = process(ctx, "100")
	}
}
