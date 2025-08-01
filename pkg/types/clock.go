// Package types provides core clock abstractions for time mocking
package types

import (
	"context"
	"time"
)

// Clock provides an abstraction over time operations for testing
type Clock interface {
	// Now returns the current time
	Now() time.Time
	// After returns a channel that delivers the current time after the duration
	After(d time.Duration) <-chan time.Time
	// Sleep blocks for the given duration
	Sleep(d time.Duration)
	// Since returns the time elapsed since t
	Since(t time.Time) time.Duration
	// NewTimer creates a new Timer
	NewTimer(d time.Duration) Timer
	// NewTicker creates a new Ticker
	NewTicker(d time.Duration) Ticker
}

// Timer provides timer operations
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// Ticker provides ticker operations
type Ticker interface {
	C() <-chan time.Time
	Stop()
	Reset(d time.Duration)
}

// RealClock implements Clock using real time operations
type RealClock struct{}

// NewRealClock creates a new real clock
func NewRealClock() Clock {
	return &RealClock{}
}

func (c *RealClock) Now() time.Time {
	return time.Now()
}

func (c *RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (c *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (c *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func (c *RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{timer: time.NewTimer(d)}
}

func (c *RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

// realTimer wraps time.Timer
type realTimer struct {
	timer *time.Timer
}

func (t *realTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *realTimer) Stop() bool {
	return t.timer.Stop()
}

func (t *realTimer) Reset(d time.Duration) bool {
	return t.timer.Reset(d)
}

// realTicker wraps time.Ticker
type realTicker struct {
	ticker *time.Ticker
}

func (t *realTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *realTicker) Stop() {
	t.ticker.Stop()
}

func (t *realTicker) Reset(d time.Duration) {
	t.ticker.Reset(d)
}

// ContextWithClock adds a clock to context
type clockKey struct{}

// WithClock adds a clock to the context
func WithClock(ctx context.Context, clock Clock) context.Context {
	return context.WithValue(ctx, clockKey{}, clock)
}

// ClockFromContext retrieves clock from context, returns RealClock if not found
func ClockFromContext(ctx context.Context) Clock {
	if clock, ok := ctx.Value(clockKey{}).(Clock); ok {
		return clock
	}
	return NewRealClock()
}