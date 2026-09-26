// Package backoff provides retry delays shared by background loops.
package backoff

import (
	"context"
	"math/rand/v2"
	"time"
)

// Exponential returns base doubled attempt times, capped at limit.
func Exponential(base, limit time.Duration, attempt int) time.Duration {
	delay := base
	for range attempt {
		if delay >= limit/2 {
			return limit
		}
		delay *= 2
	}
	return min(delay, limit)
}

// Jitter adds a random delay of up to fraction of delay.
func Jitter(delay time.Duration, fraction float64) time.Duration {
	return delay + time.Duration(rand.Float64()*fraction*float64(delay))
}

// Sleep waits for delay or until ctx is done, returning ctx.Err() in that case.
func Sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
