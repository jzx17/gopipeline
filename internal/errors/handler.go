// Package errors provides advanced error handling strategies and framework
package errors

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// Pre-defined common error types to avoid reflection overhead
var (
	timeoutErrorType = reflect.TypeOf((*interface{ Timeout() bool })(nil)).Elem()
	tempErrorType    = reflect.TypeOf((*interface{ Temporary() bool })(nil)).Elem()
)

// fastErrorTypeCheck quickly checks common error types to avoid reflection overhead
func fastErrorTypeCheck(err error) (reflect.Type, bool) {
	if err == nil {
		return nil, true
	}

	// Quick check for common types
	switch err.(type) {
	case interface{ Timeout() bool }:
		return timeoutErrorType, true
	case interface{ Temporary() bool }:
		return tempErrorType, true
	}

	// Cannot quickly identify, need to use reflection
	return nil, false
}

// getErrorTypeOptimized gets error type with fast path optimization
func getErrorTypeOptimized(err error) reflect.Type {
	if err == nil {
		return nil
	}

	// Try fast path first
	if fastType, ok := fastErrorTypeCheck(err); ok && fastType != nil {
		return fastType
	}

	// Fallback to reflection
	return reflect.TypeOf(err)
}

// ErrorHandler defines a powerful error handling interface
type ErrorHandler interface {
	// HandleError handles the error, returns processed error or nil if handled
	HandleError(ctx context.Context, errCtx *ErrorContext) error

	// Name returns the name of the error handler
	Name() string

	// CanHandle determines if it can handle specific type of error
	CanHandle(err error) bool
}

// ErrorContext defines context information when error occurs
type ErrorContext struct {
	// Error that occurred
	Error error

	// OperationName is the name of the operation where error occurred
	OperationName string

	// InputData is the input data that caused the error
	InputData interface{}

	// InputType is the type information of input data
	InputType reflect.Type

	// Timestamp when the error occurred
	Timestamp time.Time

	// RetryCount is the current retry count
	RetryCount int

	// MaxRetries is the maximum retry count
	MaxRetries int

	// ErrorChain tracks error propagation path
	ErrorChain []*ChainedError

	// Metadata contains additional metadata information
	Metadata map[string]interface{}

	// PipelineID is the unique identifier of the Pipeline instance
	PipelineID string
}

// ChainedError represents a node in the error chain
type ChainedError struct {
	// Error information
	Error error

	// Stage where the error occurred
	Stage string

	// Timestamp when the error occurred
	Timestamp time.Time

	// Duration of processing in this stage
	Duration time.Duration
}

// AddToChain adds an error to the error chain
func (ec *ErrorContext) AddToChain(err error, stage string, duration time.Duration) {
	if ec.ErrorChain == nil {
		ec.ErrorChain = make([]*ChainedError, 0)
	}

	chainedErr := &ChainedError{
		Error:     err,
		Stage:     stage,
		Timestamp: time.Now(),
		Duration:  duration,
	}

	ec.ErrorChain = append(ec.ErrorChain, chainedErr)
}

// GetRootError gets the root error in the error chain
func (ec *ErrorContext) GetRootError() error {
	if len(ec.ErrorChain) == 0 {
		return ec.Error
	}
	return ec.ErrorChain[0].Error
}

// GetLastError gets the last error in the error chain
func (ec *ErrorContext) GetLastError() error {
	if len(ec.ErrorChain) == 0 {
		return ec.Error
	}
	return ec.ErrorChain[len(ec.ErrorChain)-1].Error
}

// NewErrorContext creates a new error context
func NewErrorContext(err error, operationName string, inputData interface{}) *ErrorContext {
	var inputType reflect.Type
	if inputData != nil {
		inputType = reflect.TypeOf(inputData) // No optimization here, inputData is not error type
	}

	return &ErrorContext{
		Error:         err,
		OperationName: operationName,
		InputData:     inputData,
		InputType:     inputType,
		Timestamp:     time.Now(),
		RetryCount:    0,
		MaxRetries:    3, // Default maximum retry count
		ErrorChain:    make([]*ChainedError, 0),
		Metadata:      make(map[string]interface{}),
	}
}

// ErrorHandlerStrategy defines error handling strategy types
type ErrorHandlerStrategy int

const (
	// FailFastStrategy is the fail-fast strategy
	FailFastStrategy ErrorHandlerStrategy = iota
	// ContinueOnErrorStrategy ignores errors and continues execution
	ContinueOnErrorStrategy
)

// String returns the string representation of the strategy
func (s ErrorHandlerStrategy) String() string {
	switch s {
	case FailFastStrategy:
		return "FailFast"
	case ContinueOnErrorStrategy:
		return "ContinueOnError"
	default:
		return "Unknown"
	}
}

// FailFastHandler implements fail-fast error handling
type FailFastHandler struct {
	name string
}

// NewFailFastHandler creates a new fail-fast handler
func NewFailFastHandler() *FailFastHandler {
	return &FailFastHandler{
		name: "FailFast",
	}
}

// HandleError implements the ErrorHandler interface
func (h *FailFastHandler) HandleError(ctx context.Context, errCtx *ErrorContext) error {
	// Fail fast: return original error directly
	return errCtx.Error
}

// Name returns the handler name
func (h *FailFastHandler) Name() string {
	return h.name
}

// CanHandle checks if it can handle errors (fail-fast handler can handle all errors)
func (h *FailFastHandler) CanHandle(err error) bool {
	return true
}

// ContinueOnErrorHandler ignores errors and continues execution
type ContinueOnErrorHandler struct {
	name              string
	ignoredErrorTypes map[reflect.Type]bool
	logErrors         bool
	mu                sync.RWMutex
}

// ContinueOnErrorConfig contains configuration for continue-on-error handler
type ContinueOnErrorConfig struct {
	// IgnoredErrorTypes is the list of error types to ignore
	IgnoredErrorTypes []error
	// LogErrors determines whether to log ignored errors
	LogErrors bool
}

// NewContinueOnErrorHandler creates a continue-on-error handler
func NewContinueOnErrorHandler(config *ContinueOnErrorConfig) *ContinueOnErrorHandler {
	handler := &ContinueOnErrorHandler{
		name:              "ContinueOnError",
		ignoredErrorTypes: make(map[reflect.Type]bool),
		logErrors:         true,
	}

	if config != nil {
		handler.logErrors = config.LogErrors

		// Register error types to ignore
		for _, errType := range config.IgnoredErrorTypes {
			if errType != nil {
				handler.ignoredErrorTypes[reflect.TypeOf(errType)] = true // No optimization, errType is interface type
			}
		}
	}

	return handler
}

// HandleError implements the ErrorHandler interface
func (h *ContinueOnErrorHandler) HandleError(ctx context.Context, errCtx *ErrorContext) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Check if this is an error type to ignore (optimized: fast path check)
	errType := getErrorTypeOptimized(errCtx.Error)
	if _, ignored := h.ignoredErrorTypes[errType]; !ignored {
		// If no ignored error types configured, ignore all errors by default
		if len(h.ignoredErrorTypes) > 0 {
			return errCtx.Error // Don't ignore this type of error
		}
	}

	// Log error (if enabled)
	if h.logErrors {
		// Here we can integrate with logging system
		fmt.Printf("[%s] Ignored error in operation %s: %v\n",
			time.Now().Format(time.RFC3339), errCtx.OperationName, errCtx.Error)
	}

	// Return nil to indicate error has been handled (ignored)
	return nil
}

// Name returns the handler name
func (h *ContinueOnErrorHandler) Name() string {
	return h.name
}

// CanHandle checks if it can handle the error
func (h *ContinueOnErrorHandler) CanHandle(err error) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// If no specific error types configured, can handle all errors
	if len(h.ignoredErrorTypes) == 0 {
		return true
	}

	// Check if it's a configured error type
	errType := reflect.TypeOf(err)
	_, canHandle := h.ignoredErrorTypes[errType]
	return canHandle
}

// AddIgnoredErrorType adds an error type to ignore
func (h *ContinueOnErrorHandler) AddIgnoredErrorType(err error) {
	if err == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	errType := reflect.TypeOf(err)
	h.ignoredErrorTypes[errType] = true
}

// RemoveIgnoredErrorType removes an error type from ignore list
func (h *ContinueOnErrorHandler) RemoveIgnoredErrorType(err error) {
	if err == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	errType := reflect.TypeOf(err)
	delete(h.ignoredErrorTypes, errType)
}

// HandlerRegistry is a registry for error handlers
type HandlerRegistry struct {
	handlers       map[string]ErrorHandler
	typeHandlers   map[reflect.Type]ErrorHandler
	defaultHandler ErrorHandler
	mu             sync.RWMutex
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	// Create handler instances
	failFastHandler := NewFailFastHandler()
	continueOnErrorHandler := NewContinueOnErrorHandler(nil)

	registry := &HandlerRegistry{
		handlers:       make(map[string]ErrorHandler),
		typeHandlers:   make(map[reflect.Type]ErrorHandler),
		defaultHandler: failFastHandler, // Use fail-fast strategy by default
	}

	// Register built-in handlers (using same instances)
	registry.RegisterHandler(failFastHandler)
	registry.RegisterHandler(continueOnErrorHandler)

	return registry
}

// RegisterHandler registers an error handler
func (r *HandlerRegistry) RegisterHandler(handler ErrorHandler) error {
	if handler == nil {
		return fmt.Errorf("cannot register nil handler")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := handler.Name()
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("handler with name %s already exists", name)
	}

	r.handlers[name] = handler
	return nil
}

// UnregisterHandler unregisters an error handler
func (r *HandlerRegistry) UnregisterHandler(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; !exists {
		return fmt.Errorf("handler with name %s not found", name)
	}

	delete(r.handlers, name)

	// Clean up corresponding handlers in type mapping
	for errType, handler := range r.typeHandlers {
		if handler.Name() == name {
			delete(r.typeHandlers, errType)
		}
	}

	return nil
}

// GetHandler gets an error handler by name
func (r *HandlerRegistry) GetHandler(name string) (ErrorHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[name]
	if !exists {
		return nil, fmt.Errorf("handler with name %s not found", name)
	}

	return handler, nil
}

// GetHandlerForError gets the most suitable handler for an error type
func (r *HandlerRegistry) GetHandlerForError(err error) ErrorHandler {
	if err == nil {
		return r.defaultHandler
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// First check if there's a handler for specific error type
	errType := reflect.TypeOf(err)
	if handler, exists := r.typeHandlers[errType]; exists {
		return handler
	}

	// If no specific binding, return default handler
	return r.defaultHandler
}

// SetDefaultHandler sets the default error handler
func (r *HandlerRegistry) SetDefaultHandler(handler ErrorHandler) error {
	if handler == nil {
		return fmt.Errorf("cannot set nil as default handler")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultHandler = handler
	return nil
}

// GetDefaultHandler gets the default error handler
func (r *HandlerRegistry) GetDefaultHandler() ErrorHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.defaultHandler
}

// BindErrorTypeToHandler binds a specific error type to a handler
func (r *HandlerRegistry) BindErrorTypeToHandler(errType error, handlerName string) error {
	if errType == nil {
		return fmt.Errorf("cannot bind nil error type")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	handler, exists := r.handlers[handlerName]
	if !exists {
		return fmt.Errorf("handler with name %s not found", handlerName)
	}

	eType := reflect.TypeOf(errType)
	r.typeHandlers[eType] = handler

	return nil
}

// UnbindErrorType unbinds error type from handler
func (r *HandlerRegistry) UnbindErrorType(errType error) error {
	if errType == nil {
		return fmt.Errorf("cannot unbind nil error type")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	eType := reflect.TypeOf(errType)
	delete(r.typeHandlers, eType)

	return nil
}

// ListHandlers lists all registered handlers
func (r *HandlerRegistry) ListHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}

	return names
}

// GetTypeBindings gets all error type bindings
func (r *HandlerRegistry) GetTypeBindings() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bindings := make(map[string]string)
	for errType, handler := range r.typeHandlers {
		bindings[errType.String()] = handler.Name()
	}

	return bindings
}

// Global default registry
var (
	defaultRegistry     *HandlerRegistry
	defaultRegistryOnce sync.Once
)

// getDefaultRegistry safely gets the global default registry
func getDefaultRegistry() *HandlerRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewHandlerRegistry()
	})
	return defaultRegistry
}

// resetGlobalRegistryForTesting resets global registry (for testing only)
func resetGlobalRegistryForTesting() {
	defaultRegistry = nil
	defaultRegistryOnce = sync.Once{}
}

// RegisterGlobalHandler registers handler to global registry
func RegisterGlobalHandler(handler ErrorHandler) error {
	return getDefaultRegistry().RegisterHandler(handler)
}

// GetGlobalHandler gets handler from global registry
func GetGlobalHandler(name string) (ErrorHandler, error) {
	return getDefaultRegistry().GetHandler(name)
}

// GetGlobalHandlerForError gets handler for error from global registry
func GetGlobalHandlerForError(err error) ErrorHandler {
	return getDefaultRegistry().GetHandlerForError(err)
}

// SetGlobalDefaultHandler sets global default handler
func SetGlobalDefaultHandler(handler ErrorHandler) error {
	return getDefaultRegistry().SetDefaultHandler(handler)
}
