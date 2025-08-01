// Package pipeline provides scalable chaining composition solutions
package pipeline

import (
	"context"
	"fmt"
)

// ========== Solution 1: Dynamic Chaining Composition (Recommended) ==========

// DynamicChain scalable dynamic chaining composition
type DynamicChain[T any] struct {
	process ProcessFunc[T, T]
}

// NewDynamicChain creates a dynamic chain
func NewDynamicChain[T any]() *DynamicChain[T] {
	return &DynamicChain[T]{
		process: func(ctx context.Context, input T) (T, error) {
			return input, nil // Initially identity function
		},
	}
}

// Then adds the next processing step
func (c *DynamicChain[T]) Then(step ProcessFunc[T, T]) *DynamicChain[T] {
	prevProcess := c.process
	return &DynamicChain[T]{
		process: func(ctx context.Context, input T) (T, error) {
			intermediate, err := prevProcess(ctx, input)
			if err != nil {
				var zero T
				return zero, err // Return zero value on error
			}
			return step(ctx, intermediate)
		},
	}
}

// Execute executes the entire chain
func (c *DynamicChain[T]) Execute(ctx context.Context, input T) (T, error) {
	return c.process(ctx, input)
}

// ToFunc converts to ProcessFunc
func (c *DynamicChain[T]) ToFunc() ProcessFunc[T, T] {
	return c.process
}

// ========== Solution 2: Generic Interface Chain ==========

// Step processing step interface
type Step[T any] interface {
	Process(ctx context.Context, input T) (T, error)
	Name() string
}

// FuncStep wraps function as Step
type FuncStep[T any] struct {
	name string
	fn   ProcessFunc[T, T]
}

func NewFuncStep[T any](name string, fn ProcessFunc[T, T]) *FuncStep[T] {
	return &FuncStep[T]{name: name, fn: fn}
}

func (s *FuncStep[T]) Process(ctx context.Context, input T) (T, error) {
	return s.fn(ctx, input)
}

func (s *FuncStep[T]) Name() string {
	return s.name
}

// ChainBuilder chain builder
type ChainBuilder[T any] struct {
	steps []Step[T]
}

// NewChainBuilder creates chain builder
func NewChainBuilder[T any]() *ChainBuilder[T] {
	return &ChainBuilder[T]{
		steps: make([]Step[T], 0),
	}
}

// Add adds step
func (b *ChainBuilder[T]) Add(step Step[T]) *ChainBuilder[T] {
	b.steps = append(b.steps, step)
	return b
}

// AddFunc adds function step
func (b *ChainBuilder[T]) AddFunc(name string, fn ProcessFunc[T, T]) *ChainBuilder[T] {
	return b.Add(NewFuncStep(name, fn))
}

// Build builds the final processing function
func (b *ChainBuilder[T]) Build() ProcessFunc[T, T] {
	steps := make([]Step[T], len(b.steps))
	copy(steps, b.steps)

	return func(ctx context.Context, input T) (T, error) {
		current := input
		for i, step := range steps {
			result, err := step.Process(ctx, current)
			if err != nil {
				return current, fmt.Errorf("step %d (%s) failed: %w", i, step.Name(), err)
			}
			current = result
		}
		return current, nil
	}
}

// ========== Solution 3: Variadic Chain (Type-unsafe but simple) ==========

// UntypedStep untyped step function
type UntypedStep func(ctx context.Context, input interface{}) (interface{}, error)

// ChainVariadic variadic chaining composition
func ChainVariadic(steps ...UntypedStep) UntypedStep {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		current := input
		for i, step := range steps {
			result, err := step(ctx, current)
			if err != nil {
				return nil, fmt.Errorf("step %d failed: %w", i, err)
			}
			current = result
		}
		return current, nil
	}
}

// TypedStep type-safe step wrapper
func TypedStep[T any](fn ProcessFunc[T, T]) UntypedStep {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		typedInput, ok := input.(T)
		if !ok {
			return nil, fmt.Errorf("type assertion failed: expected %T, got %T", *new(T), input)
		}
		return fn(ctx, typedInput)
	}
}

// ========== Solution 4: Array-driven Chain (Highest Performance) ==========

// ArrayChain array-driven chaining processing
type ArrayChain[T any] struct {
	functions []ProcessFunc[T, T]
}

// NewArrayChain creates array chain
func NewArrayChain[T any](functions ...ProcessFunc[T, T]) *ArrayChain[T] {
	return &ArrayChain[T]{
		functions: functions,
	}
}

// Append appends function
func (c *ArrayChain[T]) Append(fn ProcessFunc[T, T]) *ArrayChain[T] {
	newFunctions := make([]ProcessFunc[T, T], len(c.functions)+1)
	copy(newFunctions, c.functions)
	newFunctions[len(c.functions)] = fn
	return &ArrayChain[T]{functions: newFunctions}
}

// Execute executes chain
func (c *ArrayChain[T]) Execute(ctx context.Context, input T) (T, error) {
	current := input
	for i, fn := range c.functions {
		result, err := fn(ctx, current)
		if err != nil {
			return current, fmt.Errorf("function %d failed: %w", i, err)
		}
		current = result
	}
	return current, nil
}

// ToFunc converts to single function
func (c *ArrayChain[T]) ToFunc() ProcessFunc[T, T] {
	functions := make([]ProcessFunc[T, T], len(c.functions))
	copy(functions, c.functions)

	return func(ctx context.Context, input T) (T, error) {
		current := input
		for i, fn := range functions {
			result, err := fn(ctx, current)
			if err != nil {
				return current, fmt.Errorf("function %d failed: %w", i, err)
			}
			current = result
		}
		return current, nil
	}
}

// ========== Solution 5: Functional Immutable Chain ==========

// ImmutableChain immutable chain
type ImmutableChain[T any] []ProcessFunc[T, T]

// NewImmutableChain creates immutable chain
func NewImmutableChain[T any](functions ...ProcessFunc[T, T]) ImmutableChain[T] {
	chain := make(ImmutableChain[T], len(functions))
	copy(chain, functions)
	return chain
}

// Then adds function (returns new chain)
func (c ImmutableChain[T]) Then(fn ProcessFunc[T, T]) ImmutableChain[T] {
	newChain := make(ImmutableChain[T], len(c)+1)
	copy(newChain, c)
	newChain[len(c)] = fn
	return newChain
}

// Execute executes chain
func (c ImmutableChain[T]) Execute(ctx context.Context, input T) (T, error) {
	current := input
	for i, fn := range c {
		result, err := fn(ctx, current)
		if err != nil {
			return current, fmt.Errorf("step %d failed: %w", i, err)
		}
		current = result
	}
	return current, nil
}

// ToFunc converts to function
func (c ImmutableChain[T]) ToFunc() ProcessFunc[T, T] {
	chain := make([]ProcessFunc[T, T], len(c))
	copy(chain, c)

	return func(ctx context.Context, input T) (T, error) {
		current := input
		for i, fn := range chain {
			result, err := fn(ctx, current)
			if err != nil {
				return current, fmt.Errorf("step %d failed: %w", i, err)
			}
			current = result
		}
		return current, nil
	}
}
