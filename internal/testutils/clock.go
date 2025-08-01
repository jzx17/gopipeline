package testutils

import (
	"context"
	"testing"
	"time"

	"github.com/coder/quartz"
	"github.com/jzx17/gopipeline/pkg/types"
)

// NewMockClock creates a mock clock for testing
func NewMockClock(t testing.TB) *quartz.Mock {
	return quartz.NewMock(t)
}

// ClockWrapper wraps quartz.Mock to implement our Clock interface
type ClockWrapper struct {
	*quartz.Mock
}

// NewClockWrapper creates a new ClockWrapper
func NewClockWrapper(mock *quartz.Mock) *ClockWrapper {
	return &ClockWrapper{Mock: mock}
}

// After returns a channel that delivers the current time after the duration
func (c *ClockWrapper) After(d time.Duration) <-chan time.Time {
	timer := c.Mock.NewTimer(d)
	return timer.C
}

// Sleep blocks for the given duration
func (c *ClockWrapper) Sleep(d time.Duration) {
	timer := c.Mock.NewTimer(d)
	<-timer.C
}

// Now returns the current time
func (c *ClockWrapper) Now() time.Time {
	return c.Mock.Now()
}

// Since returns the time elapsed since t
func (c *ClockWrapper) Since(t time.Time) time.Duration {
	return c.Mock.Since(t)
}

// NewTimer creates a new Timer
func (c *ClockWrapper) NewTimer(d time.Duration) types.Timer {
	timer := c.Mock.NewTimer(d)
	return &TimerWrapper{timer: timer}
}

// NewTicker creates a new Ticker  
func (c *ClockWrapper) NewTicker(d time.Duration) types.Ticker {
	ticker := c.Mock.NewTicker(d)
	return &TickerWrapper{ticker: ticker}
}

// TimerWrapper wraps quartz timer
type TimerWrapper struct {
	timer *quartz.Timer
}

func (t *TimerWrapper) C() <-chan time.Time {
	return t.timer.C
}

func (t *TimerWrapper) Stop() bool {
	return t.timer.Stop()
}

func (t *TimerWrapper) Reset(d time.Duration) bool {
	return t.timer.Reset(d)
}

// TickerWrapper wraps quartz ticker
type TickerWrapper struct {
	ticker *quartz.Ticker
}

func (t *TickerWrapper) C() <-chan time.Time {
	return t.ticker.C
}

func (t *TickerWrapper) Stop() {
	t.ticker.Stop()
}

func (t *TickerWrapper) Reset(d time.Duration) {
	t.ticker.Reset(d)
}

// WithMockClock creates a context with mock clock
func WithMockClock(ctx context.Context, mock *quartz.Mock) context.Context {
	return types.WithClock(ctx, NewClockWrapper(mock))
}