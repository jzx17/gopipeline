// Package types defines type-safe Pipeline interfaces
package types

import (
	"context"
)

// TypedPipeline defines a type-safe pipeline interface
// Recommended to use functional API (functional.go) for pipeline composition
type TypedPipeline[T any] interface {
	// Execute executes the entire pipeline
	Execute(ctx context.Context, input T) (interface{}, error)

	// ExecuteWithResult executes and returns a result of the specified type
	ExecuteWithResult(ctx context.Context, input T, result interface{}) error

	// Start starts the pipeline
	Start() error

	// Stop stops the pipeline
	Stop() error

	// Close closes the pipeline
	Close() error

	// GetState returns the pipeline state
	GetState() PipelineState
}

// TypeInfo defines type information
type TypeInfo struct {
	Name    string // Type name, e.g. "string", "int", "User"
	Kind    string // Type kind, e.g. "primitive", "struct", "slice"
	PkgPath string // Package path
}

// TypeRegistry defines the type registry interface
type TypeRegistry interface {
	// RegisterType registers type information
	RegisterType(example interface{}) TypeInfo

	// GetTypeInfo returns type information
	GetTypeInfo(example interface{}) TypeInfo

	// CanConvert checks if two types can be converted
	CanConvert(from, to TypeInfo) bool

	// Convert performs type conversion
	Convert(value interface{}, from, to TypeInfo) (interface{}, error)
}

// PipelineContext defines Pipeline execution context
type PipelineContext struct {
	// Base context
	Context context.Context

	// Type registry
	TypeRegistry TypeRegistry

	// Configuration information
	Config Config

	// Execution statistics
	Stats *ExecutionStats
}

// ExecutionStats defines execution statistics
type ExecutionStats struct {
	TotalExecutions int64
	SuccessCount    int64
	FailureCount    int64
	TotalDuration   int64 // Duration in nanoseconds, supports atomic operations
}
