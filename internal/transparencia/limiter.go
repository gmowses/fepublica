package transparencia

import (
	"context"
	"time"
)

// Limiter gates outgoing requests. A Wait call must block until it's
// safe to issue the next request, or return the context's error.
type Limiter interface {
	Wait(ctx context.Context) error
}

// SleepLimiter is a simple fixed-delay limiter: every call sleeps for at least
// `interval` since the last successful Wait.
type SleepLimiter struct {
	interval time.Duration
	last     time.Time
}

// NewSleepLimiter returns a SleepLimiter with the given minimum interval.
func NewSleepLimiter(interval time.Duration) *SleepLimiter {
	return &SleepLimiter{interval: interval}
}

// Wait implements Limiter.
func (s *SleepLimiter) Wait(ctx context.Context) error {
	now := time.Now()
	earliest := s.last.Add(s.interval)
	if now.Before(earliest) {
		delay := earliest.Sub(now)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	s.last = time.Now()
	return nil
}

// NoLimiter is a no-op limiter useful in tests.
type NoLimiter struct{}

// Wait implements Limiter.
func (NoLimiter) Wait(_ context.Context) error { return nil }
