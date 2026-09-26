package marketsync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/backoff"
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

// NewScheduler schedules every supplied supported interval. The logger must be
// non-nil.
func NewScheduler(profiles map[market.CandleInterval]Runner, logger *slog.Logger) *Scheduler {
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
	return backoff.Exponential(base, maxRetryDelay, int(attempt))
}

// NextRun returns the next scheduled synchronization time for interval.
func NextRun(interval market.CandleInterval, now time.Time) time.Time {
	open := interval.OpenTime(now)
	candidate := open.Add(scheduleDelay)
	if candidate.After(now.UTC()) {
		return candidate
	}
	return interval.NextOpenTime(open).Add(scheduleDelay)
}

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
					scheduler.logger.ErrorContext(ctx, "scheduled market synchronization failed", "module", "market_sync", "operation", "scheduled_sync", "profile", job.profile, "outcome", "failure", "retry_after", delay, "error", err)
					retryJob := job
					retryJob.retryAttempt++
					runs.Add(1)
					go func() {
						defer runs.Done()
						if backoff.Sleep(ctx, delay) == nil {
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
			timer := time.NewTimer(time.Until(NextRun(interval, time.Now())))
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					queue.enqueue(profile, runner)
					timer.Reset(time.Until(NextRun(interval, time.Now())))
				}
			}
		}()
	}

	<-ctx.Done()
	runs.Wait()
	return nil
}
