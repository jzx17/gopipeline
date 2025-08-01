package errors

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// TestErrorContext tests basic functionality of error context
func TestErrorContext(t *testing.T) {
	testErr := errors.New("test error")
	operationName := "test-operation"
	inputData := "test input"

	errCtx := NewErrorContext(testErr, operationName, inputData)

	// Verify basic fields
	if errCtx.Error != testErr {
		t.Errorf("Expected error %v, got %v", testErr, errCtx.Error)
	}
	if errCtx.OperationName != operationName {
		t.Errorf("Expected operation name %s, got %s", operationName, errCtx.OperationName)
	}
	if errCtx.InputData != inputData {
		t.Errorf("Expected input data %v, got %v", inputData, errCtx.InputData)
	}

	// Verify type information
	expectedType := reflect.TypeOf(inputData)
	if errCtx.InputType != expectedType {
		t.Errorf("Expected input type %v, got %v", expectedType, errCtx.InputType)
	}

	// Verify default values
	if errCtx.RetryCount != 0 {
		t.Errorf("Expected retry count 0, got %d", errCtx.RetryCount)
	}
	if errCtx.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", errCtx.MaxRetries)
	}
	if len(errCtx.ErrorChain) != 0 {
		t.Errorf("Expected empty error chain, got %d items", len(errCtx.ErrorChain))
	}
	if len(errCtx.Metadata) != 0 {
		t.Errorf("Expected empty metadata, got %d items", len(errCtx.Metadata))
	}
}

// TestErrorContextChain tests error chain functionality
func TestErrorContextChain(t *testing.T) {
	errCtx := NewErrorContext(errors.New("root error"), "operation1", "input")

	// Add errors to chain
	err1 := errors.New("stage1 error")
	errCtx.AddToChain(err1, "stage1", time.Millisecond*100)

	err2 := errors.New("stage2 error")
	errCtx.AddToChain(err2, "stage2", time.Millisecond*200)

	// Verify error chain
	if len(errCtx.ErrorChain) != 2 {
		t.Errorf("Expected 2 errors in chain, got %d", len(errCtx.ErrorChain))
	}

	// Verify root error
	rootErr := errCtx.GetRootError()
	if rootErr != err1 {
		t.Errorf("Expected root error %v, got %v", err1, rootErr)
	}

	// Verify last error
	lastErr := errCtx.GetLastError()
	if lastErr != err2 {
		t.Errorf("Expected last error %v, got %v", err2, lastErr)
	}

	// Verify detailed information in chain
	if errCtx.ErrorChain[0].Stage != "stage1" {
		t.Errorf("Expected stage1, got %s", errCtx.ErrorChain[0].Stage)
	}
	if errCtx.ErrorChain[1].Stage != "stage2" {
		t.Errorf("Expected stage2, got %s", errCtx.ErrorChain[1].Stage)
	}
}

// TestFailFastHandler tests fail-fast handler
func TestFailFastHandler(t *testing.T) {
	handler := NewFailFastHandler()

	// Verify basic properties
	if handler.Name() != "FailFast" {
		t.Errorf("Expected name 'FailFast', got %s", handler.Name())
	}

	// Verify can handle all errors
	testErr := errors.New("test error")
	if !handler.CanHandle(testErr) {
		t.Error("FailFastHandler should be able to handle all errors")
	}

	// Test error handling
	errCtx := NewErrorContext(testErr, "test-operation", "input")
	ctx := context.Background()

	result := handler.HandleError(ctx, errCtx)
	if result != testErr {
		t.Errorf("Expected original error %v, got %v", testErr, result)
	}
}

// TestContinueOnErrorHandler tests continue-on-error handler
func TestContinueOnErrorHandler(t *testing.T) {
	// Test default configuration
	handler := NewContinueOnErrorHandler(nil)

	if handler.Name() != "ContinueOnError" {
		t.Errorf("Expected name 'ContinueOnError', got %s", handler.Name())
	}

	// Test error handling
	testErr := errors.New("test error")
	errCtx := NewErrorContext(testErr, "test-operation", "input")
	ctx := context.Background()

	result := handler.HandleError(ctx, errCtx)
	if result != nil {
		t.Errorf("Expected nil (error ignored), got %v", result)
	}

	// Test can handle all errors (default configuration)
	if !handler.CanHandle(testErr) {
		t.Error("ContinueOnErrorHandler should be able to handle all errors by default")
	}
}

// CustomError custom error type for testing
type CustomError struct {
	msg string
}

func (e CustomError) Error() string {
	return e.msg
}

// TestContinueOnErrorHandlerWithConfig tests continue-on-error handler with configuration
func TestContinueOnErrorHandlerWithConfig(t *testing.T) {

	ignoredErr := CustomError{msg: "ignored error"}
	config := &ContinueOnErrorConfig{
		IgnoredErrorTypes: []error{ignoredErr},
		LogErrors:         false, // Disable logging to simplify test
	}

	handler := NewContinueOnErrorHandler(config)

	// Test can handle configured error type
	if !handler.CanHandle(ignoredErr) {
		t.Error("Handler should be able to handle configured error type")
	}

	// Test cannot handle unconfigured error type
	otherErr := errors.New("other error")
	if handler.CanHandle(otherErr) {
		t.Error("Handler should not be able to handle unconfigured error type")
	}

	// Test handling configured error type
	errCtx := NewErrorContext(ignoredErr, "test-operation", "input")
	ctx := context.Background()

	result := handler.HandleError(ctx, errCtx)
	if result != nil {
		t.Errorf("Expected nil (error ignored), got %v", result)
	}

	// Test handling unconfigured error type
	errCtx2 := NewErrorContext(otherErr, "test-operation", "input")
	result2 := handler.HandleError(ctx, errCtx2)
	if result2 != otherErr {
		t.Errorf("Expected original error %v, got %v", otherErr, result2)
	}
}

// TestContinueOnErrorHandlerAddRemove tests dynamic add/remove error types
func TestContinueOnErrorHandlerAddRemove(t *testing.T) {
	handler := NewContinueOnErrorHandler(nil)

	// Add custom error type
	customErr := errors.New("custom error")
	handler.AddIgnoredErrorType(customErr)

	// Verify can handle
	if !handler.CanHandle(customErr) {
		t.Error("Handler should be able to handle added error type")
	}

	// Remove error type
	handler.RemoveIgnoredErrorType(customErr)

	// Verify should now handle all errors (because list is empty)
	if !handler.CanHandle(customErr) {
		t.Error("Handler should be able to handle all errors when list is empty")
	}
}

// TestHandlerRegistry tests handler registry
func TestHandlerRegistry(t *testing.T) {
	registry := NewHandlerRegistry()

	// Verify default handler
	defaultHandler := registry.GetDefaultHandler()
	if defaultHandler == nil {
		t.Error("Default handler should not be nil")
	}
	if defaultHandler.Name() != "FailFast" {
		t.Errorf("Expected default handler name 'FailFast', got %s", defaultHandler.Name())
	}

	// Verify built-in handlers are registered
	handlers := registry.ListHandlers()
	if len(handlers) != 2 {
		t.Errorf("Expected 2 built-in handlers, got %d", len(handlers))
	}

	// Test getting handlers
	failFastHandler, err := registry.GetHandler("FailFast")
	if err != nil {
		t.Errorf("Failed to get FailFast handler: %v", err)
	}
	if failFastHandler.Name() != "FailFast" {
		t.Errorf("Expected FailFast handler, got %s", failFastHandler.Name())
	}

	continueHandler, err := registry.GetHandler("ContinueOnError")
	if err != nil {
		t.Errorf("Failed to get ContinueOnError handler: %v", err)
	}
	if continueHandler.Name() != "ContinueOnError" {
		t.Errorf("Expected ContinueOnError handler, got %s", continueHandler.Name())
	}

	// Test simple GetHandlerForError
	testErr := errors.New("simple test error")
	handler := registry.GetHandlerForError(testErr)
	t.Logf("Handler for simple error: %s", handler.Name())
	t.Logf("Default handler: %s", registry.GetDefaultHandler().Name())
	t.Logf("Are they equal: %v", handler == registry.GetDefaultHandler())
}

// TestHandlerRegistryCustomHandler tests registering custom handler
func TestHandlerRegistryCustomHandler(t *testing.T) {
	registry := NewHandlerRegistry()

	// Create custom handler
	customHandler := &mockHandler{name: "CustomHandler"}

	// Register custom handler
	err := registry.RegisterHandler(customHandler)
	if err != nil {
		t.Errorf("Failed to register custom handler: %v", err)
	}

	// Verify can retrieve
	retrieved, err := registry.GetHandler("CustomHandler")
	if err != nil {
		t.Errorf("Failed to get custom handler: %v", err)
	}
	if retrieved != customHandler {
		t.Error("Retrieved handler should be the same instance")
	}

	// Test duplicate registration
	err = registry.RegisterHandler(customHandler)
	if err == nil {
		t.Error("Should not be able to register handler with duplicate name")
	}

	// Test unregistration
	err = registry.UnregisterHandler("CustomHandler")
	if err != nil {
		t.Errorf("Failed to unregister handler: %v", err)
	}

	// Verify unregistered
	_, err = registry.GetHandler("CustomHandler")
	if err == nil {
		t.Error("Should not be able to get unregistered handler")
	}
}

// TestHandlerRegistryErrorTypeBinding tests error type binding
func TestHandlerRegistryErrorTypeBinding(t *testing.T) {
	// Create an independent registry
	failFastHandler := NewFailFastHandler()
	continueOnErrorHandler := NewContinueOnErrorHandler(nil)

	registry := &HandlerRegistry{
		handlers:       make(map[string]ErrorHandler),
		typeHandlers:   make(map[reflect.Type]ErrorHandler),
		defaultHandler: failFastHandler,
	}

	registry.handlers["FailFast"] = failFastHandler
	registry.handlers["ContinueOnError"] = continueOnErrorHandler

	// Test custom error type binding
	customErr := CustomError{msg: "custom error"}
	err := registry.BindErrorTypeToHandler(customErr, "ContinueOnError")
	if err != nil {
		t.Errorf("Failed to bind error type: %v", err)
	}

	// Test getting bound handler
	handler := registry.GetHandlerForError(customErr)
	if handler.Name() != "ContinueOnError" {
		t.Errorf("Expected ContinueOnError handler, got %s", handler.Name())
	}

	// Test getting handler for unbound error type (should return default handler)
	otherErr := errors.New("other standard error")
	handler2 := registry.GetHandlerForError(otherErr)

	if handler2.Name() != "FailFast" {
		t.Errorf("Expected default FailFast handler for unbound error type, got %s", handler2.Name())
	}

	// Test unbinding
	err = registry.UnbindErrorType(customErr)
	if err != nil {
		t.Errorf("Failed to unbind error type: %v", err)
	}

	// Verify returns default handler after unbinding
	handler3 := registry.GetHandlerForError(customErr)
	if handler3.Name() != "FailFast" {
		t.Errorf("Expected default FailFast handler after unbind, got %s", handler3.Name())
	}
}

// TestGlobalRegistry tests global registry functionality
func TestGlobalRegistry(t *testing.T) {
	// Reset global state before test starts to avoid race conditions
	resetGlobalRegistryForTesting()

	// Use defer to ensure state is reset after test
	defer resetGlobalRegistryForTesting()

	// Test global functions
	customHandler := &mockHandler{name: "GlobalTestHandler"}

	err := RegisterGlobalHandler(customHandler)
	if err != nil {
		t.Errorf("Failed to register global handler: %v", err)
	}

	retrieved, err := GetGlobalHandler("GlobalTestHandler")
	if err != nil {
		t.Errorf("Failed to get global handler: %v", err)
	}
	if retrieved != customHandler {
		t.Error("Retrieved global handler should be the same instance")
	}

	// Test global error handler retrieval
	testErr := errors.New("test error")
	handler := GetGlobalHandlerForError(testErr)
	if handler == nil {
		t.Error("Global handler for error should not be nil")
	}

	// Test setting global default handler
	err = SetGlobalDefaultHandler(customHandler)
	if err != nil {
		t.Errorf("Failed to set global default handler: %v", err)
	}

	// Verify default handler set successfully
	defaultHandler := GetGlobalHandlerForError(errors.New("any error"))
	if defaultHandler != customHandler {
		t.Error("Global default handler should be the custom handler")
	}
}

// TestErrorHandlerStrategy_String tests string representation of error handler strategy
func TestErrorHandlerStrategy_String(t *testing.T) {
	tests := []struct {
		name     string
		strategy ErrorHandlerStrategy
		expected string
	}{
		{"FailFast", FailFastStrategy, "FailFast"},
		{"ContinueOnError", ContinueOnErrorStrategy, "ContinueOnError"},
		{"Unknown", ErrorHandlerStrategy(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.strategy.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestHandlerRegistry_GetTypeBindings tests getting type bindings
func TestHandlerRegistry_GetTypeBindings(t *testing.T) {
	registry := NewHandlerRegistry()

	// Bind error types
	err1 := &types.PipelineError{}
	err2 := &types.RetryableError{}

	errBind1 := registry.BindErrorTypeToHandler(err1, "FailFast")
	if errBind1 != nil {
		t.Errorf("Failed to bind error type: %v", errBind1)
	}

	errBind2 := registry.BindErrorTypeToHandler(err2, "ContinueOnError")
	if errBind2 != nil {
		t.Errorf("Failed to bind error type: %v", errBind2)
	}

	// Get type bindings
	bindings := registry.GetTypeBindings()

	// Verify bindings exist
	if len(bindings) == 0 {
		t.Error("Should have type bindings")
	}

	// Verify binding content
	for errType, handlerName := range bindings {
		if errType == "" {
			t.Error("Error type should not be empty")
		}
		if handlerName == "" {
			t.Error("Handler name should not be empty")
		}
		if handlerName != "FailFast" && handlerName != "ContinueOnError" {
			t.Errorf("Unexpected handler name: %s", handlerName)
		}
	}
}

// mockHandler mock handler for testing
type mockHandler struct {
	name         string
	canHandleAll bool
}

func (m *mockHandler) HandleError(ctx context.Context, errCtx *ErrorContext) error {
	return errCtx.Error
}

func (m *mockHandler) Name() string {
	return m.name
}

func (m *mockHandler) CanHandle(err error) bool {
	return m.canHandleAll
}

// Benchmarks

// BenchmarkErrorContextCreation benchmark test for error context creation
func BenchmarkErrorContextCreation(b *testing.B) {
	err := errors.New("test error")
	operationName := "test-operation"
	inputData := "test input"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewErrorContext(err, operationName, inputData)
	}
}

// BenchmarkFailFastHandler benchmark test for fail-fast handler
func BenchmarkFailFastHandler(b *testing.B) {
	handler := NewFailFastHandler()
	errCtx := NewErrorContext(errors.New("test error"), "test-operation", "input")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.HandleError(ctx, errCtx)
	}
}

// BenchmarkContinueOnErrorHandler benchmark test for continue-on-error handler
func BenchmarkContinueOnErrorHandler(b *testing.B) {
	config := &ContinueOnErrorConfig{
		LogErrors: false, // Disable logging for accurate benchmark results
	}
	handler := NewContinueOnErrorHandler(config)
	errCtx := NewErrorContext(errors.New("test error"), "test-operation", "input")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.HandleError(ctx, errCtx)
	}
}

// BenchmarkHandlerRegistryLookup benchmark test for handler registry lookup
func BenchmarkHandlerRegistryLookup(b *testing.B) {
	registry := NewHandlerRegistry()
	testErr := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.GetHandlerForError(testErr)
	}
}
