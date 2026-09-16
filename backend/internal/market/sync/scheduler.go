package sync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/market"
)

// Runner is the synchronization operation scheduled at process and time boundaries.
type Runner interface {
	Sync(context.Context) error
}

// Scheduler runs startup catch-up and interval-boundary synchronization through
// one shared worker so profiles never overlap inside a process.
type Scheduler struct {
	profiles   map[market.CandleInterval]Runner
	logger     *slog.Logger
	retryDelay time.Duration
}

type scheduledJob struct {
	profile      string
	runner       Runner
	retryAttempt uint
}

type jobQueue struct {
	jobs       chan scheduledJob
	mu         sync.Mutex
	pending    map[string]bool
	generation map[string]uint64
}

func newJobQueue() *jobQueue {
	return &jobQueue{
		jobs:    make(chan scheduledJob, len(market.CandleIntervals())),
		pending: make(map[string]bool), generation: make(map[string]uint64),
	}
}

func (queue *jobQueue) enqueue(profile string, runner Runner) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.pending[profile] {
		return
	}
	queue.pending[profile] = true
	queue.jobs <- scheduledJob{profile: profile, runner: runner}
}

func (queue *jobQueue) complete(profile string) uint64 {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	delete(queue.pending, profile)
	queue.generation[profile]++
	return queue.generation[profile]
}

func (queue *jobQueue) enqueueRetry(job scheduledJob, generation uint64) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.generation[job.profile] != generation || queue.pending[job.profile] {
		return
	}
	queue.pending[job.profile] = true
	queue.jobs <- job
}

// NewScheduler creates a daily scheduler for compatibility with single-runner callers.
func NewScheduler(runner Runner, logger *slog.Logger) *Scheduler {
	return NewSchedulerWithProfiles(map[market.CandleInterval]Runner{market.IntervalDay: runner}, logger)
}

// NewSchedulerWithHourly creates the legacy daily/hourly scheduler.
func NewSchedulerWithHourly(daily, hourly Runner, logger *slog.Logger) *Scheduler {
	return NewSchedulerWithProfiles(map[market.CandleInterval]Runner{
		market.IntervalDay: daily, market.IntervalHour: hourly,
	}, logger)
}

// NewSchedulerWithProfiles schedules every supplied supported interval.
func NewSchedulerWithProfiles(profiles map[market.CandleInterval]Runner, logger *slog.Logger) *Scheduler {
	owned := make(map[market.CandleInterval]Runner, len(profiles))
	for _, interval := range market.CandleIntervals() {
		if profiles[interval] != nil {
			owned[interval] = profiles[interval]
		}
	}
	return &Scheduler{profiles: owned, logger: logger, retryDelay: time.Minute}
}

const (
	scheduleDelay = 30 * time.Second
	maxRetryDelay = 15 * time.Minute
)

func retryDelay(base time.Duration, attempt uint) time.Duration {
	delay := base
	for range attempt {
		if delay >= maxRetryDelay/2 {
			return maxRetryDelay
		}
		delay *= 2
	}
	return min(delay, maxRetryDelay)
}

func nextIntervalRun(interval market.CandleInterval, now time.Time) time.Time {
	open := interval.OpenTime(now)
	candidate := open.Add(scheduleDelay)
	if candidate.After(now.UTC()) {
		return candidate
	}
	return interval.NextOpenTime(open).Add(scheduleDelay)
}

func NextHourlyRun(now time.Time) time.Time  { return nextIntervalRun(market.IntervalHour, now) }
func NextDailyRun(now time.Time) time.Time   { return nextIntervalRun(market.IntervalDay, now) }
func NextWeeklyRun(now time.Time) time.Time  { return nextIntervalRun(market.IntervalWeek, now) }
func NextMonthlyRun(now time.Time) time.Time { return nextIntervalRun(market.IntervalMonth, now) }

// Run starts catch-up work without blocking startup, schedules UTC-boundary
// work, and waits for its worker and timer goroutines during cancellation.
func (scheduler *Scheduler) Run(ctx context.Context) error {
	queue := newJobQueue()
	var runs sync.WaitGroup
	runs.Add(1)
	go func() {
		defer runs.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-queue.jobs:
				if ctx.Err() != nil {
					queue.complete(job.profile)
					return
				}
				err := job.runner.Sync(ctx)
				generation := queue.complete(job.profile)
				if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrSyncInProgress) {
					delay := retryDelay(scheduler.retryDelay, job.retryAttempt)
					scheduler.logger.ErrorContext(ctx, "scheduled market synchronization failed", "module", "market_sync", "operation", "scheduled_sync", "profile", job.profile, "outcome", "failure", "retry_after", delay, "error", err.Error())
					retryJob := job
					retryJob.retryAttempt++
					runs.Add(1)
					go func() {
						defer runs.Done()
						timer := time.NewTimer(delay)
						defer stopTimer(timer)
						select {
						case <-ctx.Done():
							return
						case <-timer.C:
							queue.enqueueRetry(retryJob, generation)
						}
					}()
				}
			}
		}
	}()

	startupOrder := []market.CandleInterval{market.IntervalDay, market.IntervalHour, market.IntervalWeek, market.IntervalMonth}
	for _, interval := range startupOrder {
		runner := scheduler.profiles[interval]
		if runner == nil {
			continue
		}
		profile := string(interval)
		queue.enqueue(profile, runner)
		runs.Add(1)
		go func() {
			defer runs.Done()
			timer := time.NewTimer(time.Until(nextIntervalRun(interval, time.Now())))
			defer stopTimer(timer)
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					queue.enqueue(profile, runner)
					resetTimer(timer, time.Until(nextIntervalRun(interval, time.Now())))
				}
			}
		}()
	}

	<-ctx.Done()
	runs.Wait()
	return nil
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
