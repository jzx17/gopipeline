// Package retry provides retry mechanism strategies and implementations
package retry

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// RetryPolicy defines the retry strategy interface
type RetryPolicy interface {
	// ShouldRetry determines whether to retry
	ShouldRetry(err error, attempt int) bool

	// NextDelay returns the delay for the next retry
	NextDelay(attempt int) time.Duration

	// MaxAttempts returns the maximum retry attempts
	MaxAttempts() int

	// Reset resets the policy state (for multiple retry scenarios)
	Reset()
}

// RetryCondition is a function that determines retry conditions
type RetryCondition func(error) bool

// BaseRetryPolicy provides common retry functionality
type BaseRetryPolicy struct {
	maxAttempts    int
	retryCondition RetryCondition
	jitter         bool
	jitterFactor   float64
	mu             sync.RWMutex
}

// NewBaseRetryPolicy creates a base retry policy
func NewBaseRetryPolicy(maxAttempts int, opts ...PolicyOption) *BaseRetryPolicy {
	policy := &BaseRetryPolicy{
		maxAttempts:    maxAttempts,
		retryCondition: DefaultRetryCondition,
		jitter:         false,
		jitterFactor:   0.1,
	}

	for _, opt := range opts {
		opt(policy)
	}

	return policy
}

// ShouldRetry determines whether to retry
func (p *BaseRetryPolicy) ShouldRetry(err error, attempt int) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if attempt >= p.maxAttempts {
		return false
	}

	return p.retryCondition(err)
}

// MaxAttempts returns the maximum retry attempts
func (p *BaseRetryPolicy) MaxAttempts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxAttempts
}

// Reset resets the policy state
func (p *BaseRetryPolicy) Reset() {
	// base policy is stateless, no reset needed
}

// applyJitter applies jitter to delay
func (p *BaseRetryPolicy) applyJitter(delay time.Duration) time.Duration {
	if !p.jitter {
		return delay
	}

	jitterRange := float64(delay) * p.jitterFactor
	jitterAmount := (rand.Float64() - 0.5) * 2 * jitterRange

	result := delay + time.Duration(jitterAmount)
	if result < 0 {
		result = delay / 2
	}

	return result
}

// FixedDelayRetry implements fixed delay retry strategy
type FixedDelayRetry struct {
	*BaseRetryPolicy
	delay time.Duration
}

// NewFixedDelayRetry creates a fixed delay retry policy
func NewFixedDelayRetry(maxAttempts int, delay time.Duration, opts ...PolicyOption) *FixedDelayRetry {
	return &FixedDelayRetry{
		BaseRetryPolicy: NewBaseRetryPolicy(maxAttempts, opts...),
		delay:           delay,
	}
}

// NextDelay returns the delay for the next retry
func (p *FixedDelayRetry) NextDelay(attempt int) time.Duration {
	return p.applyJitter(p.delay)
}

// ExponentialBackoffRetry implements exponential backoff retry strategy
type ExponentialBackoffRetry struct {
	*BaseRetryPolicy
	initialDelay time.Duration
	multiplier   float64
	maxDelay     time.Duration
}

// NewExponentialBackoffRetry creates an exponential backoff retry policy
func NewExponentialBackoffRetry(maxAttempts int, initialDelay time.Duration, opts ...BackoffOption) *ExponentialBackoffRetry {
	policy := &ExponentialBackoffRetry{
		BaseRetryPolicy: NewBaseRetryPolicy(maxAttempts),
		initialDelay:    initialDelay,
		multiplier:      2.0,
		maxDelay:        30 * time.Second,
	}

	for _, opt := range opts {
		opt.apply(policy)
	}

	return policy
}

// NextDelay returns the delay for the next retry
func (p *ExponentialBackoffRetry) NextDelay(attempt int) time.Duration {
	delay := time.Duration(float64(p.initialDelay) * math.Pow(p.multiplier, float64(attempt-1)))
	if delay > p.maxDelay {
		delay = p.maxDelay
	}
	return p.applyJitter(delay)
}

// LinearBackoffRetry implements linear backoff retry strategy
type LinearBackoffRetry struct {
	*BaseRetryPolicy
	initialDelay time.Duration
	increment    time.Duration
	maxDelay     time.Duration
}

// NewLinearBackoffRetry creates a linear backoff retry policy
func NewLinearBackoffRetry(maxAttempts int, initialDelay, increment time.Duration, opts ...BackoffOption) *LinearBackoffRetry {
	policy := &LinearBackoffRetry{
		BaseRetryPolicy: NewBaseRetryPolicy(maxAttempts),
		initialDelay:    initialDelay,
		increment:       increment,
		maxDelay:        30 * time.Second,
	}

	for _, opt := range opts {
		if bo, ok := opt.(*backoffOption); ok && bo.maxDelay != nil {
			policy.maxDelay = *bo.maxDelay
		}
	}

	return policy
}

// NextDelay returns the delay for the next retry
func (p *LinearBackoffRetry) NextDelay(attempt int) time.Duration {
	delay := p.initialDelay + time.Duration(attempt-1)*p.increment
	if delay > p.maxDelay {
		delay = p.maxDelay
	}
	return p.applyJitter(delay)
}

// CustomRetry implements custom retry strategy
type CustomRetry struct {
	*BaseRetryPolicy
	delayFunc DelayFunc
}

// DelayFunc is a custom delay calculation function
type DelayFunc func(attempt int) time.Duration

// NewCustomRetry creates a custom retry policy
func NewCustomRetry(maxAttempts int, delayFunc DelayFunc, opts ...PolicyOption) *CustomRetry {
	return &CustomRetry{
		BaseRetryPolicy: NewBaseRetryPolicy(maxAttempts, opts...),
		delayFunc:       delayFunc,
	}
}

// NextDelay returns the delay for the next retry
func (p *CustomRetry) NextDelay(attempt int) time.Duration {
	delay := p.delayFunc(attempt)
	return p.applyJitter(delay)
}

// PolicyOption is a configuration option for retry policies
type PolicyOption func(*BaseRetryPolicy)

// WithRetryCondition sets the retry condition
func WithRetryCondition(condition RetryCondition) PolicyOption {
	return func(p *BaseRetryPolicy) {
		p.retryCondition = condition
	}
}

// WithJitter enables jitter
func WithJitter(enabled bool, factor float64) PolicyOption {
	return func(p *BaseRetryPolicy) {
		p.jitter = enabled
		if factor > 0 && factor <= 1.0 {
			p.jitterFactor = factor
		}
	}
}

// BackoffOption is a configuration option for backoff strategies
type BackoffOption interface {
	apply(interface{})
}

type backoffOption struct {
	multiplier *float64
	maxDelay   *time.Duration
}

func (o *backoffOption) apply(policy interface{}) {
	switch p := policy.(type) {
	case *ExponentialBackoffRetry:
		if o.multiplier != nil {
			p.multiplier = *o.multiplier
		}
		if o.maxDelay != nil {
			p.maxDelay = *o.maxDelay
		}
	case *LinearBackoffRetry:
		if o.maxDelay != nil {
			p.maxDelay = *o.maxDelay
		}
	}
}

// WithMultiplier sets the multiplier for exponential backoff
func WithMultiplier(multiplier float64) BackoffOption {
	return &backoffOption{multiplier: &multiplier}
}

// WithMaxDelay sets the maximum delay time
func WithMaxDelay(maxDelay time.Duration) BackoffOption {
	return &backoffOption{maxDelay: &maxDelay}
}

// DefaultRetryCondition is the default retry condition
func DefaultRetryCondition(err error) bool {
	if err == nil {
		return false
	}

	// check if it's a RetryableError
	if types.IsRetryable(err) {
		return true
	}

	// check common retryable errors
	return isRetryableError(err)
}

// RetryableErrorTypes checks for retryable error types
func RetryableErrorTypes(err error) bool {
	// check for timeout errors
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false // context-related errors are not retried
	}

	// check for Pipeline-related retryable errors
	switch err {
	case types.ErrTimeout, types.ErrWorkerPoolFull:
		return true
	case types.ErrPipelineClosed, types.ErrPipelineStopped, types.ErrStreamClosed:
		return false // state-related errors are not retried
	default:
		return false
	}
}

// NetworkErrorTypes is a retry condition for network error types
func NetworkErrorTypes(err error) bool {
	// this can be extended to check specific network error types
	// such as DNS errors, connection timeouts, temporary network failures
	return DefaultRetryCondition(err)
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	// check underlying error in PipelineError
	if pipelineErr, ok := err.(*types.PipelineError); ok {
		return DefaultRetryCondition(pipelineErr.Cause)
	}

	return RetryableErrorTypes(err)
}
