package types

import (
	"errors"
	"testing"
	"time"
)

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrPipelineClosed", ErrPipelineClosed},
		{"ErrPipelineStopped", ErrPipelineStopped},
		{"ErrInvalidInput", ErrInvalidInput},
		{"ErrTimeout", ErrTimeout},
		{"ErrFiltered", ErrFiltered},
		{"ErrWorkerPoolFull", ErrWorkerPoolFull},
		{"ErrStreamClosed", ErrStreamClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error, got nil")
			}
			if tt.err.Error() == "" {
				t.Errorf("expected non-empty error message")
			}
		})
	}
}

func TestTypedPipelineError(t *testing.T) {
	t.Run("Type Safe Error", func(t *testing.T) {
		originalErr := errors.New("original error")
		pipelineErr := NewTypedPipelineError("validation", "test input", originalErr)

		if pipelineErr.Operation != "validation" {
			t.Errorf("expected operation 'validation', got %q", pipelineErr.Operation)
		}

		if pipelineErr.Input != "test input" {
			t.Errorf("expected input 'test input', got %v", pipelineErr.Input)
		}

		if pipelineErr.Cause != originalErr {
			t.Errorf("expected cause to be original error")
		}

		expectedMsg := "pipeline error in operation validation: original error"
		if pipelineErr.Error() != expectedMsg {
			t.Errorf("expected message %q, got %q", expectedMsg, pipelineErr.Error())
		}
	})

	t.Run("Numeric Input Type", func(t *testing.T) {
		originalErr := errors.New("numeric error")
		pipelineErr := NewTypedPipelineError("computation", 42, originalErr)

		// Ensure type safety
		if pipelineErr.Input != 42 {
			t.Errorf("expected input 42, got %v", pipelineErr.Input)
		}

		// Input is type-safe int, not interface{}
		var _ int = pipelineErr.Input
	})

	t.Run("Custom Struct Type", func(t *testing.T) {
		type testData struct {
			ID   int
			Name string
		}

		data := testData{ID: 1, Name: "test"}
		originalErr := errors.New("struct error")
		pipelineErr := NewTypedPipelineError("process", data, originalErr)

		if pipelineErr.Input.ID != 1 || pipelineErr.Input.Name != "test" {
			t.Errorf("expected input to preserve struct fields")
		}

		// Input is type-safe testData, not interface{}
		var _ testData = pipelineErr.Input
	})
}

func TestPipelineError(t *testing.T) {
	t.Run("Basic Error", func(t *testing.T) {
		originalErr := errors.New("original error")
		pipelineErr := NewPipelineError("validation", "test input", originalErr)

		if pipelineErr.Operation != "validation" {
			t.Errorf("expected operation 'validation', got %q", pipelineErr.Operation)
		}

		if pipelineErr.Input != "test input" {
			t.Errorf("expected input 'test input', got %v", pipelineErr.Input)
		}

		if pipelineErr.Cause != originalErr {
			t.Errorf("expected cause to be original error")
		}

		expectedMsg := "pipeline error in operation validation: original error"
		if pipelineErr.Error() != expectedMsg {
			t.Errorf("expected message %q, got %q", expectedMsg, pipelineErr.Error())
		}
	})

	t.Run("Error Unwrapping", func(t *testing.T) {
		originalErr := errors.New("original error")
		pipelineErr := NewPipelineError("transform", "data", originalErr)

		unwrapped := errors.Unwrap(pipelineErr)
		if unwrapped != originalErr {
			t.Errorf("expected unwrapped error to be original error")
		}
	})

	t.Run("Error Is", func(t *testing.T) {
		originalErr := ErrTimeout
		pipelineErr := NewPipelineError("process", "input", originalErr)

		if !errors.Is(pipelineErr, ErrTimeout) {
			t.Errorf("expected error to be ErrTimeout")
		}

		if errors.Is(pipelineErr, ErrInvalidInput) {
			t.Errorf("expected error not to be ErrInvalidInput")
		}
	})

	t.Run("WithContext", func(t *testing.T) {
		pipelineErr := NewPipelineError("test", "input", errors.New("error"))
		pipelineErr.WithContext("retry_count", 3)
		pipelineErr.WithContext("timestamp", time.Now())

		if len(pipelineErr.Context) != 2 {
			t.Errorf("expected 2 context items, got %d", len(pipelineErr.Context))
		}

		if pipelineErr.Context["retry_count"] != 3 {
			t.Errorf("expected retry_count to be 3, got %v", pipelineErr.Context["retry_count"])
		}
	})
}

func TestRetryableError(t *testing.T) {
	t.Run("Retryable Error", func(t *testing.T) {
		originalErr := errors.New("network error")
		retryableErr := &RetryableError{
			Err:        originalErr,
			Retryable:  true,
			RetryAfter: 5 * time.Second,
		}

		if retryableErr.Error() != originalErr.Error() {
			t.Errorf("expected error message to match original")
		}

		if errors.Unwrap(retryableErr) != originalErr {
			t.Errorf("expected unwrapped error to be original")
		}

		if !IsRetryable(retryableErr) {
			t.Errorf("expected error to be retryable")
		}

		if GetRetryDelay(retryableErr) != 5*time.Second {
			t.Errorf("expected retry delay to be 5 seconds")
		}
	})

	t.Run("Non-Retryable Error", func(t *testing.T) {
		retryableErr := &RetryableError{
			Err:       errors.New("validation error"),
			Retryable: false,
		}

		if IsRetryable(retryableErr) {
			t.Errorf("expected error not to be retryable")
		}

		if GetRetryDelay(retryableErr) != 0 {
			t.Errorf("expected retry delay to be 0")
		}
	})

	t.Run("Regular Error", func(t *testing.T) {
		regularErr := errors.New("regular error")

		if IsRetryable(regularErr) {
			t.Errorf("expected regular error not to be retryable")
		}

		if GetRetryDelay(regularErr) != 0 {
			t.Errorf("expected retry delay to be 0 for regular error")
		}
	})
}
