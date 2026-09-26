package marketsync

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

func TestRetryDelayUsesBoundedExponentialBackoff(t *testing.T) {
	for _, test := range []struct {
		attempt uint
		want    time.Duration
	}{
		{attempt: 0, want: time.Minute},
		{attempt: 1, want: 2 * time.Minute},
		{attempt: 4, want: 15 * time.Minute},
		{attempt: 100, want: 15 * time.Minute},
	} {
		if got := retryDelay(time.Minute, test.attempt); got != test.want {
			t.Errorf("retryDelay(_, %d) = %s, want %s", test.attempt, got, test.want)
		}
	}
}

func TestSchedulerRetriesFailedStartupProfile(t *testing.T) {
	runner := &failOnceRunner{succeeded: make(chan struct{})}
	scheduler := NewScheduler(map[market.CandleInterval]Runner{market.IntervalDay: runner}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	scheduler.retryDelay = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- scheduler.Run(ctx) }()

	select {
	case <-runner.succeeded:
	case <-time.After(time.Second):
		t.Fatal("failed startup synchronization was not retried")
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("Scheduler.Run() error = %v", err)
	}
	if got := runner.callCount(); got != 2 {
		t.Fatalf("Sync() calls = %d, want 2", got)
	}
}

type failOnceRunner struct {
	mu        sync.Mutex
	calls     int
	succeeded chan struct{}
}

func (runner *failOnceRunner) Sync(context.Context) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	runner.calls++
	if runner.calls == 1 {
		return errors.New("temporary failure")
	}
	if runner.calls == 2 {
		close(runner.succeeded)
	}
	return nil
}

func (runner *failOnceRunner) callCount() int {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.calls
}
