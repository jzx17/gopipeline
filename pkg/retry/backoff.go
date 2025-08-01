// Package retry provides backoff algorithm implementations
package retry

import (
	"math"
	"math/rand"
	"time"
)

// BackoffStrategy defines the backoff strategy interface
type BackoffStrategy interface {
	// NextDelay calculates the delay for the next retry
	NextDelay(attempt int) time.Duration

	// Reset resets the backoff state
	Reset()
}

// FixedBackoff implements fixed backoff strategy
type FixedBackoff struct {
	delay  time.Duration
	jitter JitterFunc
}

// NewFixedBackoff creates a fixed backoff strategy
func NewFixedBackoff(delay time.Duration, opts ...BackoffStrategyOption) *FixedBackoff {
	b := &FixedBackoff{
		delay: delay,
	}

	for _, opt := range opts {
		opt.applyToFixed(b)
	}

	return b
}

// NextDelay calculates the delay for the next retry
func (b *FixedBackoff) NextDelay(attempt int) time.Duration {
	delay := b.delay
	if b.jitter != nil {
		delay = b.jitter(delay)
	}
	return delay
}

// Reset resets the backoff state
func (b *FixedBackoff) Reset() {
	// fixed backoff is stateless, no reset needed
}

// ExponentialBackoff implements exponential backoff strategy
type ExponentialBackoff struct {
	initialDelay time.Duration
	multiplier   float64
	maxDelay     time.Duration
	jitter       JitterFunc
}

// NewExponentialBackoff creates an exponential backoff strategy
func NewExponentialBackoff(initialDelay time.Duration, opts ...BackoffStrategyOption) *ExponentialBackoff {
	b := &ExponentialBackoff{
		initialDelay: initialDelay,
		multiplier:   2.0,
		maxDelay:     30 * time.Second,
	}

	for _, opt := range opts {
		opt.applyToExponential(b)
	}

	return b
}

// NextDelay calculates the delay for the next retry
func (b *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	// calculate exponential backoff delay
	delay := time.Duration(float64(b.initialDelay) * math.Pow(b.multiplier, float64(attempt-1)))

	// limit maximum delay
	if delay > b.maxDelay {
		delay = b.maxDelay
	}

	// apply jitter
	if b.jitter != nil {
		delay = b.jitter(delay)
	}

	return delay
}

// Reset resets the backoff state
func (b *ExponentialBackoff) Reset() {
	// Exponential backoff is stateless, no reset needed
}

// LinearBackoff implements linear backoff strategy
type LinearBackoff struct {
	initialDelay time.Duration
	increment    time.Duration
	maxDelay     time.Duration
	jitter       JitterFunc
}

// NewLinearBackoff creates a linear backoff strategy
func NewLinearBackoff(initialDelay, increment time.Duration, opts ...BackoffStrategyOption) *LinearBackoff {
	b := &LinearBackoff{
		initialDelay: initialDelay,
		increment:    increment,
		maxDelay:     30 * time.Second,
	}

	for _, opt := range opts {
		opt.applyToLinear(b)
	}

	return b
}

// NextDelay calculates the delay for the next retry
func (b *LinearBackoff) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	// calculate linear growth delay
	delay := b.initialDelay + time.Duration(attempt-1)*b.increment

	// limit maximum delay
	if delay > b.maxDelay {
		delay = b.maxDelay
	}

	// apply jitter
	if b.jitter != nil {
		delay = b.jitter(delay)
	}

	return delay
}

// Reset resets the backoff state
func (b *LinearBackoff) Reset() {
	// Linear backoff is stateless, no reset needed
}

// FibonacciBackoff implements fibonacci backoff strategy
type FibonacciBackoff struct {
	baseDelay time.Duration
	maxDelay  time.Duration
	jitter    JitterFunc
	fibCache  []int64 // Cache Fibonacci sequence
}

// NewFibonacciBackoff creates a fibonacci backoff strategy
func NewFibonacciBackoff(baseDelay time.Duration, opts ...BackoffStrategyOption) *FibonacciBackoff {
	b := &FibonacciBackoff{
		baseDelay: baseDelay,
		maxDelay:  30 * time.Second,
		fibCache:  []int64{1, 1}, // Initialize first two Fibonacci numbers
	}

	for _, opt := range opts {
		opt.applyToFibonacci(b)
	}

	return b
}

// NextDelay calculates the delay for the next retry
func (b *FibonacciBackoff) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	// Get Fibonacci number
	fibNum := b.getFibonacci(attempt - 1)

	// Calculate delay time
	delay := time.Duration(fibNum) * b.baseDelay

	// limit maximum delay
	if delay > b.maxDelay {
		delay = b.maxDelay
	}

	// apply jitter
	if b.jitter != nil {
		delay = b.jitter(delay)
	}

	return delay
}

// getFibonacci gets the nth Fibonacci number (with cache optimization)
func (b *FibonacciBackoff) getFibonacci(n int) int64 {
	if n < len(b.fibCache) {
		return b.fibCache[n]
	}

	// Extend cache to nth number
	for i := len(b.fibCache); i <= n; i++ {
		next := b.fibCache[i-1] + b.fibCache[i-2]
		// Prevent overflow
		if next < b.fibCache[i-1] {
			next = math.MaxInt32
		}
		b.fibCache = append(b.fibCache, next)
	}

	return b.fibCache[n]
}

// Reset resets the backoff state
func (b *FibonacciBackoff) Reset() {
	// Keep basic cache to avoid recalculation
	b.fibCache = b.fibCache[:2]
}

// DecorrelatedJitterBackoff decorrelated jitter backoff strategy
// Reference AWS SDK implementation to avoid thundering herd problem
type DecorrelatedJitterBackoff struct {
	baseDelay time.Duration
	capDelay  time.Duration
	prevDelay time.Duration
}

// NewDecorrelatedJitterBackoff creates a decorrelated jitter backoff strategy
func NewDecorrelatedJitterBackoff(baseDelay, capDelay time.Duration) *DecorrelatedJitterBackoff {
	return &DecorrelatedJitterBackoff{
		baseDelay: baseDelay,
		capDelay:  capDelay,
		prevDelay: baseDelay,
	}
}

// NextDelay calculates the delay for the next retry
func (b *DecorrelatedJitterBackoff) NextDelay(attempt int) time.Duration {
	// Decorrelated jitter algorithm: random(base, prev_delay * 3)
	maxDelay := b.prevDelay * 3
	if maxDelay > b.capDelay {
		maxDelay = b.capDelay
	}

	if maxDelay <= b.baseDelay {
		b.prevDelay = b.baseDelay
		return b.baseDelay
	}

	// Randomly select within [baseDelay, maxDelay] range
	diff := maxDelay - b.baseDelay
	jitter := time.Duration(rand.Int63n(int64(diff)))
	delay := b.baseDelay + jitter

	b.prevDelay = delay
	return delay
}

// Reset resets the backoff state
func (b *DecorrelatedJitterBackoff) Reset() {
	b.prevDelay = b.baseDelay
}

// JitterFunc jitter function type
type JitterFunc func(time.Duration) time.Duration

// FullJitter full jitter function - random within [0, delay] range
func FullJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(delay)))
}

// EqualJitter equal jitter function - delay/2 + random(0, delay/2)
func EqualJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	half := delay / 2
	return half + time.Duration(rand.Int63n(int64(half)))
}

// ExponentialJitter exponential jitter function - jitter based on normal distribution
func ExponentialJitter(factor float64) JitterFunc {
	return func(delay time.Duration) time.Duration {
		if delay <= 0 {
			return 0
		}

		// Use exponential distribution to generate jitter
		jitterRange := float64(delay) * factor
		jitter := rand.ExpFloat64() * jitterRange

		result := delay + time.Duration(jitter)
		// Ensure no negative delay is generated
		if result < 0 {
			result = delay / 2
		}

		return result
	}
}

// BackoffStrategyOption backoff strategy configuration option
type BackoffStrategyOption interface {
	applyToFixed(*FixedBackoff)
	applyToExponential(*ExponentialBackoff)
	applyToLinear(*LinearBackoff)
	applyToFibonacci(*FibonacciBackoff)
}

type backoffStrategyOption struct {
	multiplier *float64
	maxDelay   *time.Duration
	jitter     JitterFunc
}

func (o *backoffStrategyOption) applyToFixed(b *FixedBackoff) {
	if o.jitter != nil {
		b.jitter = o.jitter
	}
}

func (o *backoffStrategyOption) applyToExponential(b *ExponentialBackoff) {
	if o.multiplier != nil {
		b.multiplier = *o.multiplier
	}
	if o.maxDelay != nil {
		b.maxDelay = *o.maxDelay
	}
	if o.jitter != nil {
		b.jitter = o.jitter
	}
}

func (o *backoffStrategyOption) applyToLinear(b *LinearBackoff) {
	if o.maxDelay != nil {
		b.maxDelay = *o.maxDelay
	}
	if o.jitter != nil {
		b.jitter = o.jitter
	}
}

func (o *backoffStrategyOption) applyToFibonacci(b *FibonacciBackoff) {
	if o.maxDelay != nil {
		b.maxDelay = *o.maxDelay
	}
	if o.jitter != nil {
		b.jitter = o.jitter
	}
}

// WithBackoffMultiplier sets backoff multiplier (exponential backoff only)
func WithBackoffMultiplier(multiplier float64) BackoffStrategyOption {
	return &backoffStrategyOption{multiplier: &multiplier}
}

// WithBackoffMaxDelay sets maximum delay time
func WithBackoffMaxDelay(maxDelay time.Duration) BackoffStrategyOption {
	return &backoffStrategyOption{maxDelay: &maxDelay}
}

// WithBackoffJitter sets jitter function
func WithBackoffJitter(jitter JitterFunc) BackoffStrategyOption {
	return &backoffStrategyOption{jitter: jitter}
}
