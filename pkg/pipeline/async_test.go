package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

func TestDefaultAsyncOptions(t *testing.T) {
	opts := DefaultAsyncOptions()

	if opts.BufferSize != 100 {
		t.Errorf("expected BufferSize 100, got %d", opts.BufferSize)
	}

	if opts.Concurrency != 10 {
		t.Errorf("expected Concurrency 10, got %d", opts.Concurrency)
	}
}

func TestNewAsyncPipeline(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	basePipeline := New[string, string](processFunc, nil)

	t.Run("with default options", func(t *testing.T) {
		asyncPipeline := NewAsyncPipeline(basePipeline)

		if asyncPipeline.pipeline != basePipeline {
			t.Error("expected pipeline to be set correctly")
		}

		if asyncPipeline.options.BufferSize != 100 {
			t.Errorf("expected default BufferSize 100, got %d", asyncPipeline.options.BufferSize)
		}
	})

	t.Run("with custom options", func(t *testing.T) {
		customOpts := &AsyncOptions{
			BufferSize:  50,
			Concurrency: 5,
		}

		asyncPipeline := NewAsyncPipeline(basePipeline, customOpts)

		if asyncPipeline.options.BufferSize != 50 {
			t.Errorf("expected BufferSize 50, got %d", asyncPipeline.options.BufferSize)
		}

		if asyncPipeline.options.Concurrency != 5 {
			t.Errorf("expected Concurrency 5, got %d", asyncPipeline.options.Concurrency)
		}
	})

	t.Run("with nil options", func(t *testing.T) {
		asyncPipeline := NewAsyncPipeline(basePipeline, nil)

		if asyncPipeline.options.BufferSize != 100 {
			t.Errorf("expected default BufferSize 100, got %d", asyncPipeline.options.BufferSize)
		}
	})
}

func TestAsyncPipeline_ExecuteAsync(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			return input + "-async", nil
		}

		basePipeline := New[string, string](processFunc, nil)
		asyncPipeline := NewAsyncPipeline(basePipeline)

		resultChan := asyncPipeline.ExecuteAsync(context.Background(), "test")
		result := <-resultChan

		if result.Error != nil {
			t.Errorf("unexpected error: %v", result.Error)
		}

		if result.Value != "test-async" {
			t.Errorf("expected 'test-async', got '%s'", result.Value)
		}

		if result.Duration <= 0 {
			t.Error("expected positive duration")
		}
	})

	t.Run("execution with error", func(t *testing.T) {
		expectedErr := errors.New("process error")
		processFunc := func(ctx context.Context, input string) (string, error) {
			return "", expectedErr
		}

		// Disable retries for fast testing
		config := &types.Config{
			Workers:      1,
			BufferSize:   100,
			Timeout:      30 * time.Second,
			StageTimeout: 10 * time.Second,
			MaxRetries:   0, // No retries for fast tests
			RetryDelay:   0,
			Clock:        types.NewRealClock(),
		}

		basePipeline := New[string, string](processFunc, config)
		asyncPipeline := NewAsyncPipeline(basePipeline)

		resultChan := asyncPipeline.ExecuteAsync(context.Background(), "test")
		result := <-resultChan

		if result.Error == nil {
			t.Error("expected error, got nil")
		}

		if result.Value != "" {
			t.Errorf("expected empty value on error, got '%s'", result.Value)
		}
	})
}

func TestAsyncPipeline_ExecuteBatch(t *testing.T) {
	t.Run("batch execution", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			time.Sleep(10 * time.Millisecond) // Simulate processing time
			return input + "-batch", nil
		}

		basePipeline := New[string, string](processFunc, nil)
		asyncPipeline := NewAsyncPipeline(basePipeline)

		inputs := []string{"item1", "item2", "item3"}
		resultChan := asyncPipeline.ExecuteBatch(context.Background(), inputs)

		results := make(map[int]string)
		errorCount := 0

		for result := range resultChan {
			if result.Error != nil {
				errorCount++
			} else {
				results[result.Index] = result.Value
			}
		}

		if errorCount > 0 {
			t.Errorf("expected no errors, got %d", errorCount)
		}

		if len(results) != len(inputs) {
			t.Errorf("expected %d results, got %d", len(inputs), len(results))
		}

		for i, input := range inputs {
			expected := input + "-batch"
			if results[i] != expected {
				t.Errorf("for input[%d], expected '%s', got '%s'", i, expected, results[i])
			}
		}
	})

	t.Run("batch with some errors", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			if input == "error" {
				return "", errors.New("failed")
			}
			return input + "-success", nil
		}

		// Disable retries for fast testing
		config := &types.Config{
			Workers:      1,
			BufferSize:   100,
			Timeout:      30 * time.Second,
			StageTimeout: 10 * time.Second,
			MaxRetries:   0, // No retries for fast tests
			RetryDelay:   0,
			Clock:        types.NewRealClock(),
		}

		basePipeline := New[string, string](processFunc, config)
		asyncPipeline := NewAsyncPipeline(basePipeline)

		inputs := []string{"item1", "error", "item3"}
		resultChan := asyncPipeline.ExecuteBatch(context.Background(), inputs)

		successCount := 0
		errorCount := 0

		for result := range resultChan {
			if result.Error != nil {
				errorCount++
			} else {
				successCount++
			}
		}

		if successCount != 2 {
			t.Errorf("expected 2 successes, got %d", successCount)
		}

		if errorCount != 1 {
			t.Errorf("expected 1 error, got %d", errorCount)
		}
	})
}

func TestAsyncPipeline_Close(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	basePipeline := New[string, string](processFunc, nil)
	asyncPipeline := NewAsyncPipeline(basePipeline)

	err := asyncPipeline.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}
