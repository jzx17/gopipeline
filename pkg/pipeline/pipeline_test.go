package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

func TestPipelineBasicFunctionality(t *testing.T) {
	// Create configuration
	config := &types.Config{
		Workers:      2,
		BufferSize:   10,
		Timeout:      5 * time.Second,
		StageTimeout: 2 * time.Second,
		MaxRetries:   3,
		RetryDelay:   100 * time.Millisecond,
	}

	// Create processing function
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	// Create Pipeline
	p := New[string, string](processFunc, config)

	// Test state management
	t.Run("State Management", func(t *testing.T) {
		// Initial state should be Created
		if p.GetState() != types.StateCreated {
			t.Errorf("Expected initial state to be Created, got %v", p.GetState())
		}

		// Start Pipeline
		if err := p.Start(); err != nil {
			t.Errorf("Failed to start pipeline: %v", err)
		}

		if p.GetState() != types.StateRunning {
			t.Errorf("Expected state to be Running after start, got %v", p.GetState())
		}

		// Stop Pipeline
		if err := p.Stop(); err != nil {
			t.Errorf("Failed to stop pipeline: %v", err)
		}

		if p.GetState() != types.StateStopped {
			t.Errorf("Expected state to be Stopped after stop, got %v", p.GetState())
		}

		// Close Pipeline
		if err := p.Close(); err != nil {
			t.Errorf("Failed to close pipeline: %v", err)
		}

		if p.GetState() != types.StateClosed {
			t.Errorf("Expected state to be Closed after close, got %v", p.GetState())
		}
	})
}

func TestPipelineExecution(t *testing.T) {
	config := &types.Config{
		Workers:    1,
		MaxRetries: 1,
	}

	t.Run("Successful Execution", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			return input + "-success", nil
		}

		p := New[string, string](processFunc, config)

		result, err := p.Execute(context.Background(), "test")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if result != "test-success" {
			t.Errorf("Expected result 'test-success', got '%s'", result)
		}
	})

	t.Run("Execution with Error", func(t *testing.T) {
		expectedErr := errors.New("process error")
		processFunc := func(ctx context.Context, input string) (string, error) {
			return "", expectedErr
		}

		p := New[string, string](processFunc, config)

		_, err := p.Execute(context.Background(), "test")
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("Execution with Timeout", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			select {
			case <-time.After(2 * time.Second):
				return input + "-delayed", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		config := &types.Config{
			Timeout: 100 * time.Millisecond,
		}

		p := New[string, string](processFunc, config)

		_, err := p.Execute(context.Background(), "test")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected DeadlineExceeded error, got: %v", err)
		}
	})
}

func TestPipelineAsyncExecution(t *testing.T) {
	config := &types.Config{
		Workers: 1,
	}

	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-async", nil
	}

	p := New[string, string](processFunc, config)

	t.Run("Async Execution", func(t *testing.T) {
		resultChan := p.ExecuteAsync(context.Background(), "test")

		result := <-resultChan
		if result.Error != nil {
			t.Errorf("Unexpected error: %v", result.Error)
		}

		if result.Value != "test-async" {
			t.Errorf("Expected result 'test-async', got '%s'", result.Value)
		}

		if result.Duration <= 0 {
			t.Error("Expected positive duration")
		}
	})
}

func TestPipelineBatchExecution(t *testing.T) {
	config := &types.Config{
		Workers: 2,
	}

	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-batch", nil
	}

	p := New[string, string](processFunc, config)

	t.Run("Batch Execution", func(t *testing.T) {
		inputs := []string{"item1", "item2", "item3"}
		resultChan := p.ExecuteBatch(context.Background(), inputs)

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
			t.Errorf("Expected no errors, got %d", errorCount)
		}

		if len(results) != len(inputs) {
			t.Errorf("Expected %d results, got %d", len(inputs), len(results))
		}

		// Verify results
		for i, input := range inputs {
			expected := input + "-batch"
			if results[i] != expected {
				t.Errorf("For input[%d], expected '%s', got '%s'", i, expected, results[i])
			}
		}
	})
}

func TestPipelineRetry(t *testing.T) {
	attemptCount := 0
	processFunc := func(ctx context.Context, input string) (string, error) {
		attemptCount++
		if attemptCount < 3 {
			return "", errors.New("temporary error")
		}
		return input + "-retried", nil
	}

	config := &types.Config{
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	}

	p := New[string, string](processFunc, config)

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Errorf("Unexpected error after retries: %v", err)
	}

	if result != "test-retried" {
		t.Errorf("Expected 'test-retried', got '%s'", result)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestPipelineWithClosedState(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	p := New[string, string](processFunc, nil)

	// Close Pipeline
	if err := p.Close(); err != nil {
		t.Fatalf("Failed to close pipeline: %v", err)
	}

	// Try to execute in closed state
	_, err := p.Execute(context.Background(), "test")
	if !errors.Is(err, types.ErrPipelineClosed) {
		t.Errorf("Expected ErrPipelineClosed, got: %v", err)
	}
}

func TestPipelineStateString(t *testing.T) {
	tests := []struct {
		state    PipelineState
		expected string
	}{
		{StateCreated, "Created"},
		{StateRunning, "Running"},
		{StateStopped, "Stopped"},
		{StateClosed, "Closed"},
		{PipelineState(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPipelineWithOptions(t *testing.T) {
	t.Run("WithRetryPolicy", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			return input + "-processed", nil
		}

		// Create a simple retry policy
		retryPolicy := &mockRetryPolicy{}

		p := New[string, string](processFunc, nil, WithRetryPolicy[string, string](retryPolicy))

		// Check if retry policy is set correctly
		pipeline := p.(*pipeline[string, string])
		if pipeline.retryPolicy != retryPolicy {
			t.Error("retry policy not set correctly")
		}
	})

	t.Run("WithErrorHandler", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			return input + "-processed", nil
		}

		errorHandler := func(err error) error {
			return err
		}

		p := New[string, string](processFunc, nil, WithErrorHandler[string, string](errorHandler))

		// Check if error handler is set correctly
		pipeline := p.(*pipeline[string, string])
		if pipeline.errorHandler == nil {
			t.Error("error handler not set correctly")
		}
	})

	t.Run("WithWorkerPool", func(t *testing.T) {
		processFunc := func(ctx context.Context, input string) (string, error) {
			return input + "-processed", nil
		}

		workerPool := &mockWorkerPool{}

		p := New[string, string](processFunc, nil, WithWorkerPool[string, string](workerPool))

		// Check if worker pool is set correctly
		pipeline := p.(*pipeline[string, string])
		if pipeline.workerPool != workerPool {
			t.Error("worker pool not set correctly")
		}
	})
}

func TestPipelineAdvancedStart(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	t.Run("start from created state", func(t *testing.T) {
		p := New[string, string](processFunc, nil)

		if err := p.Start(); err != nil {
			t.Errorf("unexpected error starting pipeline: %v", err)
		}

		if p.GetState() != types.StateRunning {
			t.Errorf("expected state Running, got %v", p.GetState())
		}
	})

	t.Run("start already running pipeline", func(t *testing.T) {
		p := New[string, string](processFunc, nil)

		// Start first
		if err := p.Start(); err != nil {
			t.Fatalf("failed to start pipeline: %v", err)
		}

		// Start again, should return nil (already running)
		if err := p.Start(); err != nil {
			t.Errorf("expected nil for already running pipeline, got: %v", err)
		}
	})

	t.Run("start closed pipeline", func(t *testing.T) {
		p := New[string, string](processFunc, nil)

		// Close first
		if err := p.Close(); err != nil {
			t.Fatalf("failed to close pipeline: %v", err)
		}

		// Try to start, should return error
		err := p.Start()
		if err == nil {
			t.Error("expected error when starting closed pipeline")
		}
	})
}

func TestPipelineAdvancedStop(t *testing.T) {
	processFunc := func(ctx context.Context, input string) (string, error) {
		return input + "-processed", nil
	}

	t.Run("stop from running state", func(t *testing.T) {
		p := New[string, string](processFunc, nil)

		// Start first
		if err := p.Start(); err != nil {
			t.Fatalf("failed to start pipeline: %v", err)
		}

		// Stop
		if err := p.Stop(); err != nil {
			t.Errorf("unexpected error stopping pipeline: %v", err)
		}

		if p.GetState() != types.StateStopped {
			t.Errorf("expected state Stopped, got %v", p.GetState())
		}
	})

	t.Run("stop from created state", func(t *testing.T) {
		p := New[string, string](processFunc, nil)

		// Stop directly (not started)
		if err := p.Stop(); err != nil {
			t.Errorf("unexpected error stopping created pipeline: %v", err)
		}

		if p.GetState() != types.StateStopped {
			t.Errorf("expected state Stopped, got %v", p.GetState())
		}
	})

	t.Run("stop closed pipeline", func(t *testing.T) {
		p := New[string, string](processFunc, nil)

		// Close first
		if err := p.Close(); err != nil {
			t.Fatalf("failed to close pipeline: %v", err)
		}

		// Try to stop, should return error
		err := p.Stop()
		if !errors.Is(err, types.ErrPipelineClosed) {
			t.Errorf("expected ErrPipelineClosed, got: %v", err)
		}
	})
}

// Mock implementations for testing
type mockRetryPolicy struct{}

func (m *mockRetryPolicy) ShouldRetry(err error, attempt int) bool {
	return attempt < 3
}

func (m *mockRetryPolicy) NextDelay(attempt int) time.Duration {
	return time.Millisecond * 10
}

func (m *mockRetryPolicy) Reset() {}

type mockWorkerPool struct{}

func (m *mockWorkerPool) Submit(task types.Task) error                                   { return nil }
func (m *mockWorkerPool) SubmitWithTimeout(task types.Task, timeout time.Duration) error { return nil }
func (m *mockWorkerPool) Start(ctx context.Context) error                                { return nil }
func (m *mockWorkerPool) Stop() error                                                    { return nil }
func (m *mockWorkerPool) Close() error                                                   { return nil }
func (m *mockWorkerPool) Size() int                                                      { return 1 }
func (m *mockWorkerPool) Stats() types.WorkerPoolStats {
	return types.WorkerPoolStats{
		PoolSize:      1,
		ActiveWorkers: 1,
		QueueSize:     0,
		QueueCapacity: 10,
	}
}
