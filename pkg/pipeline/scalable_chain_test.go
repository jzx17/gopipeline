package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestDynamicChain(t *testing.T) {
	ctx := context.Background()

	t.Run("Single step processing", func(t *testing.T) {
		chain := NewDynamicChain[int]().
			Then(func(ctx context.Context, n int) (int, error) {
				return n * 2, nil
			})

		result, err := chain.Execute(ctx, 5)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != 10 {
			t.Errorf("Expected 10, got %d", result)
		}
	})

	t.Run("Multi-step processing", func(t *testing.T) {
		chain := NewDynamicChain[int]().
			Then(func(ctx context.Context, n int) (int, error) { return n + 1, nil }).
			Then(func(ctx context.Context, n int) (int, error) { return n * 2, nil }).
			Then(func(ctx context.Context, n int) (int, error) { return n - 3, nil }).
			Then(func(ctx context.Context, n int) (int, error) { return n * 10, nil }).
			Then(func(ctx context.Context, n int) (int, error) { return n + 5, nil })

		// 5 -> 6 -> 12 -> 9 -> 90 -> 95
		result, err := chain.Execute(ctx, 5)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != 95 {
			t.Errorf("Expected 95, got %d", result)
		}
	})

	t.Run("Error handling", func(t *testing.T) {
		chain := NewDynamicChain[int]().
			Then(func(ctx context.Context, n int) (int, error) { return n + 1, nil }).
			Then(func(ctx context.Context, n int) (int, error) {
				return 0, errors.New("Step 2 failed")
			}).
			Then(func(ctx context.Context, n int) (int, error) { return n * 2, nil })

		result, err := chain.Execute(ctx, 5)
		if err == nil {
			t.Error("Expected error but none occurred")
		}
		if result != 0 { // Should return zero value on error
			t.Errorf("Expected 0, got %d", result)
		}
	})
}

func TestChainBuilder(t *testing.T) {
	ctx := context.Background()

	t.Run("Build and execute", func(t *testing.T) {
		builder := NewChainBuilder[string]().
			AddFunc("upper", func(ctx context.Context, s string) (string, error) {
				return s + "_UPPER", nil
			}).
			AddFunc("suffix", func(ctx context.Context, s string) (string, error) {
				return s + "_SUFFIX", nil
			})

		process := builder.Build()
		result, err := process(ctx, "test")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		expected := "test_UPPER_SUFFIX"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("Step error information", func(t *testing.T) {
		builder := NewChainBuilder[int]().
			AddFunc("step1", func(ctx context.Context, n int) (int, error) { return n + 1, nil }).
			AddFunc("failing_step", func(ctx context.Context, n int) (int, error) {
				return 0, errors.New("Calculation error")
			})

		process := builder.Build()
		_, err := process(ctx, 5)
		if err == nil {
			t.Error("Expected error but none occurred")
		}

		expectedMsg := "step 1 (failing_step) failed"
		if err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
		}
	})
}

func TestArrayChain(t *testing.T) {
	ctx := context.Background()

	t.Run("Create and execute", func(t *testing.T) {
		chain := NewArrayChain(
			func(ctx context.Context, n int) (int, error) { return n * 2, nil },
			func(ctx context.Context, n int) (int, error) { return n + 10, nil },
			func(ctx context.Context, n int) (int, error) { return n / 3, nil },
		)

		// 5 -> 10 -> 20 -> 6
		result, err := chain.Execute(ctx, 5)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != 6 {
			t.Errorf("Expected 6, got %d", result)
		}
	})

	t.Run("Dynamic addition", func(t *testing.T) {
		chain := NewArrayChain(
			func(ctx context.Context, n int) (int, error) { return n + 1, nil },
		)

		chain = chain.Append(func(ctx context.Context, n int) (int, error) { return n * 5, nil })
		chain = chain.Append(func(ctx context.Context, n int) (int, error) { return n - 2, nil })

		// 3 -> 4 -> 20 -> 18
		result, err := chain.Execute(ctx, 3)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != 18 {
			t.Errorf("Expected 18, got %d", result)
		}
	})

	t.Run("Convert to function", func(t *testing.T) {
		chain := NewArrayChain(
			func(ctx context.Context, s string) (string, error) { return s + "A", nil },
			func(ctx context.Context, s string) (string, error) { return s + "B", nil },
		)

		fn := chain.ToFunc()
		result, err := fn(ctx, "test")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != "testAB" {
			t.Errorf("Expected 'testAB', got '%s'", result)
		}
	})
}

func TestImmutableChain(t *testing.T) {
	ctx := context.Background()

	t.Run("Basic functionality", func(t *testing.T) {
		chain := NewImmutableChain(
			func(ctx context.Context, n int) (int, error) { return n + 10, nil },
			func(ctx context.Context, n int) (int, error) { return n * 2, nil },
		)

		// 5 -> 15 -> 30
		result, err := chain.Execute(ctx, 5)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != 30 {
			t.Errorf("Expected 30, got %d", result)
		}
	})

	t.Run("Immutability", func(t *testing.T) {
		originalChain := NewImmutableChain(
			func(ctx context.Context, n int) (int, error) { return n + 1, nil },
		)

		extendedChain := originalChain.Then(
			func(ctx context.Context, n int) (int, error) { return n * 10, nil },
		)

		// Original chain should remain unchanged
		result1, err := originalChain.Execute(ctx, 5)
		if err != nil {
			t.Fatalf("Original chain execution failed: %v", err)
		}
		if result1 != 6 {
			t.Errorf("Original chain expected 6, got %d", result1)
		}

		// Extended chain should have new behavior
		result2, err := extendedChain.Execute(ctx, 5)
		if err != nil {
			t.Fatalf("Extended chain execution failed: %v", err)
		}
		if result2 != 60 {
			t.Errorf("Extended chain expected 60, got %d", result2)
		}
	})

	t.Run("Chain extension", func(t *testing.T) {
		chain := NewImmutableChain[string]().
			Then(func(ctx context.Context, s string) (string, error) { return s + "1", nil }).
			Then(func(ctx context.Context, s string) (string, error) { return s + "2", nil }).
			Then(func(ctx context.Context, s string) (string, error) { return s + "3", nil }).
			Then(func(ctx context.Context, s string) (string, error) { return s + "4", nil }).
			Then(func(ctx context.Context, s string) (string, error) { return s + "5", nil })

		result, err := chain.Execute(ctx, "test")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if result != "test12345" {
			t.Errorf("Expected 'test12345', got '%s'", result)
		}
	})
}

// Benchmark tests
func BenchmarkChainComparison(b *testing.B) {
	ctx := context.Background()
	stepFunc := func(ctx context.Context, n int) (int, error) { return n + 1, nil }

	b.Run("DynamicChain", func(b *testing.B) {
		chain := NewDynamicChain[int]().
			Then(stepFunc).Then(stepFunc).Then(stepFunc).Then(stepFunc).Then(stepFunc)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = chain.Execute(ctx, i)
		}
	})

	b.Run("ArrayChain", func(b *testing.B) {
		chain := NewArrayChain(stepFunc, stepFunc, stepFunc, stepFunc, stepFunc)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = chain.Execute(ctx, i)
		}
	})

	b.Run("ImmutableChain", func(b *testing.B) {
		chain := NewImmutableChain(stepFunc, stepFunc, stepFunc, stepFunc, stepFunc)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = chain.Execute(ctx, i)
		}
	})

	b.Run("DynamicChainAsFunc", func(b *testing.B) {
		chain := NewDynamicChain[int]().
			Then(stepFunc).Then(stepFunc).Then(stepFunc).Then(stepFunc).ToFunc()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = chain(ctx, i)
		}
	})
}

// Complex scenario tests
func TestDynamicChain_ToFunc(t *testing.T) {
	chain := NewDynamicChain[string]().
		Then(func(ctx context.Context, s string) (string, error) {
			return s + "-step1", nil
		}).
		Then(func(ctx context.Context, s string) (string, error) {
			return s + "-step2", nil
		})

	fn := chain.ToFunc()

	result, err := fn(context.Background(), "hello")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "hello-step1-step2" {
		t.Errorf("expected 'hello-step1-step2', got '%s'", result)
	}
}

func TestArrayChain_ToFunc(t *testing.T) {
	chain := NewArrayChain[string]().
		Append(func(ctx context.Context, s string) (string, error) {
			return s + "-step1", nil
		}).
		Append(func(ctx context.Context, s string) (string, error) {
			return s + "-step2", nil
		})

	fn := chain.ToFunc()

	result, err := fn(context.Background(), "start")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "start-step1-step2" {
		t.Errorf("expected 'start-step1-step2', got '%s'", result)
	}
}

func TestImmutableChain_ToFunc(t *testing.T) {
	chain := NewImmutableChain[string]().
		Then(func(ctx context.Context, s string) (string, error) {
			return s + "-immutable", nil
		})

	fn := chain.ToFunc()

	result, err := fn(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "test-immutable" {
		t.Errorf("expected 'test-immutable', got '%s'", result)
	}
}

func TestChainVariadic(t *testing.T) {
	step1 := func(ctx context.Context, s string) (string, error) {
		return s + "-1", nil
	}
	step2 := func(ctx context.Context, s string) (string, error) {
		return s + "-2", nil
	}
	step3 := func(ctx context.Context, s string) (string, error) {
		return s + "-3", nil
	}

	fn := ChainVariadic(
		TypedStep(step1),
		TypedStep(step2),
		TypedStep(step3),
	)

	result, err := fn(context.Background(), "start")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Errorf("expected string result, got %T", result)
		return
	}

	if resultStr != "start-1-2-3" {
		t.Errorf("expected 'start-1-2-3', got '%s'", resultStr)
	}
}

func TestTypedStep(t *testing.T) {
	step := TypedStep(func(ctx context.Context, s string) (string, error) {
		return fmt.Sprintf("len:%d", len(s)), nil
	})

	result, err := step(context.Background(), "hello")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Errorf("expected string result, got %T", result)
		return
	}

	expected := "len:5"
	if resultStr != expected {
		t.Errorf("expected '%s', got '%s'", expected, resultStr)
	}
}

func TestComplexScenarios(t *testing.T) {
	ctx := context.Background()

	t.Run("Data processing pipeline", func(t *testing.T) {
		// Simulate a data processing pipeline：Parse -> Validate -> Transform -> Format -> Output
		pipeline := NewDynamicChain[string]().
			Then(func(ctx context.Context, input string) (string, error) {
				// Parse
				if input == "" {
					return "", errors.New("Input is empty")
				}
				return "parsed:" + input, nil
			}).
			Then(func(ctx context.Context, input string) (string, error) {
				// Validate
				if len(input) < 10 {
					return "", errors.New("Data too short")
				}
				return "validated:" + input, nil
			}).
			Then(func(ctx context.Context, input string) (string, error) {
				// Transform
				return "transformed:" + input, nil
			}).
			Then(func(ctx context.Context, input string) (string, error) {
				// Format
				return fmt.Sprintf("formatted:[%s]", input), nil
			}).
			Then(func(ctx context.Context, input string) (string, error) {
				// Output
				return "output:" + input, nil
			})

		result, err := pipeline.Execute(ctx, "test_data")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		expected := "output:formatted:[transformed:validated:parsed:test_data]"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("Large-scale chain processing", func(t *testing.T) {
		// Create 100-step processing chain
		chain := NewDynamicChain[int]()

		for i := 0; i < 100; i++ {
			step := i // Capture loop variable
			chain = chain.Then(func(ctx context.Context, n int) (int, error) {
				return n + step, nil
			})
		}

		// Calculate expected result: 0 + (0+1+2+...+99) = 0 + 4950 = 4950
		result, err := chain.Execute(ctx, 0)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		expected := 4950
		if result != expected {
			t.Errorf("Expected %d, got %d", expected, result)
		}
	})
}
