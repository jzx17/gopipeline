package types

import (
	"context"
	"testing"
	"time"
)

func TestPipelineState_String(t *testing.T) {
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

func TestResult(t *testing.T) {
	t.Run("Successful Result", func(t *testing.T) {
		result := Result[string]{
			Value:    "test",
			Error:    nil,
			Duration: 100 * time.Millisecond,
		}

		if result.Value != "test" {
			t.Errorf("expected value 'test', got %q", result.Value)
		}

		if result.Error != nil {
			t.Errorf("expected nil error, got %v", result.Error)
		}

		if result.Duration != 100*time.Millisecond {
			t.Errorf("expected duration 100ms, got %v", result.Duration)
		}
	})
}

func TestBatchResult(t *testing.T) {
	t.Run("Batch Result", func(t *testing.T) {
		result := BatchResult[string]{
			Index:    1,
			Value:    "test",
			Error:    nil,
			Duration: 50 * time.Millisecond,
		}

		if result.Index != 1 {
			t.Errorf("expected index 1, got %d", result.Index)
		}

		if result.Value != "test" {
			t.Errorf("expected value 'test', got %q", result.Value)
		}

		if result.Error != nil {
			t.Errorf("expected nil error, got %v", result.Error)
		}

		if result.Duration != 50*time.Millisecond {
			t.Errorf("expected duration 50ms, got %v", result.Duration)
		}
	})
}

// Mock implementations for testing
type mockPipeline struct {
	state PipelineState
}

func (m *mockPipeline) Execute(ctx context.Context, input string) (string, error) {
	return input + "-processed", nil
}

func (m *mockPipeline) ExecuteAsync(ctx context.Context, input string) <-chan Result[string] {
	ch := make(chan Result[string], 1)
	ch <- Result[string]{Value: input + "-async", Error: nil, Duration: time.Millisecond}
	close(ch)
	return ch
}

func (m *mockPipeline) ExecuteBatch(ctx context.Context, inputs []string) <-chan BatchResult[string] {
	ch := make(chan BatchResult[string], len(inputs))
	for i, input := range inputs {
		ch <- BatchResult[string]{Index: i, Value: input + "-batch", Error: nil, Duration: time.Millisecond}
	}
	close(ch)
	return ch
}

func (m *mockPipeline) Start() error {
	m.state = StateRunning
	return nil
}

func (m *mockPipeline) Stop() error {
	m.state = StateStopped
	return nil
}

func (m *mockPipeline) Close() error {
	m.state = StateClosed
	return nil
}

func (m *mockPipeline) GetState() PipelineState {
	return m.state
}

func TestPipelineInterface(t *testing.T) {
	pipeline := &mockPipeline{state: StateCreated}

	t.Run("Execute", func(t *testing.T) {
		result, err := pipeline.Execute(context.Background(), "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "test-processed" {
			t.Errorf("expected 'test-processed', got %q", result)
		}
	})

	t.Run("ExecuteAsync", func(t *testing.T) {
		resultChan := pipeline.ExecuteAsync(context.Background(), "test")
		result := <-resultChan
		if result.Error != nil {
			t.Errorf("unexpected error: %v", result.Error)
		}
		if result.Value != "test-async" {
			t.Errorf("expected 'test-async', got %q", result.Value)
		}
	})

	t.Run("ExecuteBatch", func(t *testing.T) {
		inputs := []string{"item1", "item2"}
		resultChan := pipeline.ExecuteBatch(context.Background(), inputs)

		results := make([]BatchResult[string], 0, len(inputs))
		for result := range resultChan {
			results = append(results, result)
		}

		if len(results) != len(inputs) {
			t.Errorf("expected %d results, got %d", len(inputs), len(results))
		}
	})

	t.Run("State Management", func(t *testing.T) {
		if pipeline.GetState() != StateCreated {
			t.Errorf("expected initial state Created, got %v", pipeline.GetState())
		}

		if err := pipeline.Start(); err != nil {
			t.Errorf("unexpected error starting: %v", err)
		}
		if pipeline.GetState() != StateRunning {
			t.Errorf("expected state Running, got %v", pipeline.GetState())
		}

		if err := pipeline.Stop(); err != nil {
			t.Errorf("unexpected error stopping: %v", err)
		}
		if pipeline.GetState() != StateStopped {
			t.Errorf("expected state Stopped, got %v", pipeline.GetState())
		}

		if err := pipeline.Close(); err != nil {
			t.Errorf("unexpected error closing: %v", err)
		}
		if pipeline.GetState() != StateClosed {
			t.Errorf("expected state Closed, got %v", pipeline.GetState())
		}
	})
}
