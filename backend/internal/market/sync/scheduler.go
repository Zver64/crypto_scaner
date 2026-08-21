package sync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Runner is the synchronization operation scheduled at process and time boundaries.
type Runner interface {
	Sync(context.Context) error
}

// Scheduler runs catch-up synchronization and UTC boundary-aligned daily and hourly work.
type Scheduler struct {
	runner Runner
	hourly Runner
	logger *slog.Logger
}

// NewScheduler creates a daily scheduler for one synchronization runner.
func NewScheduler(runner Runner, logger *slog.Logger) *Scheduler {
	return &Scheduler{runner: runner, logger: logger}
}

// NewSchedulerWithHourly schedules daily and hourly synchronization together.
func NewSchedulerWithHourly(daily, hourly Runner, logger *slog.Logger) *Scheduler {
	return &Scheduler{runner: daily, hourly: hourly, logger: logger}
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

// NextHourlyRun returns the next minute-30 hourly boundary strictly after now.
func NextHourlyRun(now time.Time) time.Time {
	utc := now.UTC()
	next := time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), 0, 30, 0, time.UTC)
	if !next.After(utc) {
		next = next.Add(time.Hour)
	}
	return next
}

// Run starts catch-up work without blocking startup, schedules UTC daily and hourly work,
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
	dailyTimer := time.NewTimer(time.Until(NextDailyRun(time.Now())))
	var hourlyTimer *time.Timer
	var startHourly func()
	if scheduler.hourly != nil {
		hourlyTimer = time.NewTimer(time.Until(NextHourlyRun(time.Now())))
		startHourly = func() {
			runs.Add(1)
			go func() {
				defer runs.Done()
				if err := scheduler.hourly.Sync(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrSyncInProgress) {
					scheduler.logger.ErrorContext(ctx, "scheduled hourly market synchronization failed", "module", "market_sync", "operation", "scheduled_sync", "outcome", "failure", "error", err.Error())
				}
			}()
		}
		startHourly()
	}
	for {
		var hourlyChannel <-chan time.Time
		if hourlyTimer != nil {
			hourlyChannel = hourlyTimer.C
		}
		select {
		case <-ctx.Done():
			stopTimer(dailyTimer)
			if hourlyTimer != nil {
				stopTimer(hourlyTimer)
			}
			runs.Wait()
			return nil
		case <-dailyTimer.C:
			start()
			resetTimer(dailyTimer, time.Until(NextDailyRun(time.Now())))
		case <-hourlyChannel:
			startHourly()
			resetTimer(hourlyTimer, time.Until(NextHourlyRun(time.Now())))
		}
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
