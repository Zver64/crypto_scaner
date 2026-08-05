package sync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Runner is the synchronization operation scheduled at process and day boundaries.
type Runner interface {
	Sync(context.Context) error
}

// Scheduler runs catch-up synchronization and UTC boundary-aligned daily work.
type Scheduler struct {
	runner Runner
	logger *slog.Logger
}

// NewScheduler creates a daily scheduler for one synchronization runner.
func NewScheduler(runner Runner, logger *slog.Logger) *Scheduler {
	return &Scheduler{runner: runner, logger: logger}
}

// NextDailyRun returns the next 00:00:30 UTC strictly after now.
func NextDailyRun(now time.Time) time.Time {
	utc := now.UTC()
	next := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 30, 0, time.UTC)
	if !next.After(utc) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// Run starts catch-up work without blocking startup, schedules UTC daily work,
// and waits for owned synchronization goroutines during cancellation.
func (scheduler *Scheduler) Run(ctx context.Context) error {
	var runs sync.WaitGroup
	start := func() {
		runs.Add(1)
		go func() {
			defer runs.Done()
			if err := scheduler.runner.Sync(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrSyncInProgress) {
				scheduler.logger.ErrorContext(ctx, "scheduled market synchronization failed",
					"module", "market_sync", "operation", "scheduled_sync", "outcome", "failure", "error", err.Error())
			}
		}()
	}

	start()
	for {
		next := NextDailyRun(time.Now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			runs.Wait()
			return nil
		case <-timer.C:
			start()
		}
	}
}
