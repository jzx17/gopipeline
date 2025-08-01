// Package testutils provides simplified testing utilities and helper functions
package testutils

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig test configuration
type TestConfig struct {
	Timeout       time.Duration
	Workers       int
	BufferSize    int
	EnableMetrics bool
	EnableLogging bool
}

// TestContext simplified test context
type TestContext struct {
	t       *testing.T
	config  *TestConfig
	cleanup []func()
	mu      sync.RWMutex
}

// NewTestContext creates new test context
func NewTestContext(t *testing.T, config *TestConfig) *TestContext {
	if config == nil {
		config = &TestConfig{
			Timeout:       5 * time.Second,
			Workers:       2,
			BufferSize:    10,
			EnableMetrics: true,
			EnableLogging: false,
		}
	}

	return &TestContext{
		t:       t,
		config:  config,
		cleanup: make([]func(), 0),
	}
}

// T returns testing.T instance
func (tc *TestContext) T() *testing.T {
	return tc.t
}

// Context returns context with timeout
func (tc *TestContext) Context() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), tc.config.Timeout)
	tc.AddCleanup(cancel)
	return ctx
}

// AddCleanup adds cleanup function
func (tc *TestContext) AddCleanup(fn func()) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cleanup = append(tc.cleanup, fn)
}

// Cleanup executes cleanup
func (tc *TestContext) Cleanup() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Execute cleanup functions in reverse order
	for i := len(tc.cleanup) - 1; i >= 0; i-- {
		tc.cleanup[i]()
	}
	tc.cleanup = nil
}

// RequireNoError asserts no error
func (tc *TestContext) RequireNoError(err error, msgAndArgs ...interface{}) {
	if !assert.NoError(tc.t, err, msgAndArgs...) {
		tc.t.FailNow()
	}
}

// AssertEventually waits for condition to be true
func (tc *TestContext) AssertEventually(condition func() bool, timeout, tick time.Duration, msgAndArgs ...interface{}) {
	assert.Eventually(tc.t, condition, timeout, tick, msgAndArgs...)
}
