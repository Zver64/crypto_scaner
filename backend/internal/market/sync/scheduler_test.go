package sync_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	marketsync "crypto-scanner/internal/market/sync"
)

func TestNextDailyRunTargetsThirtySecondsAfterUTCMidnight(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "during prior UTC day",
			now:  time.Date(2026, time.August, 5, 5, 59, 59, 0, time.FixedZone("ICT", 7*60*60)),
			want: time.Date(2026, time.August, 5, 0, 0, 30, 0, time.UTC),
		},
		{
			name: "after today's boundary",
			now:  time.Date(2026, time.August, 5, 0, 0, 31, 0, time.UTC),
			want: time.Date(2026, time.August, 6, 0, 0, 30, 0, time.UTC),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := marketsync.NextDailyRun(test.now); !got.Equal(test.want) {
				t.Fatalf("NextDailyRun(%s) = %s, want %s", test.now, got, test.want)
			}
		})
	}
}

func TestNextHourlyRunTargetsThirtySecondsAfterUTCClockHour(t *testing.T) {
	now := time.Date(2026, time.August, 5, 5, 59, 59, 0, time.FixedZone("ICT", 7*60*60))
	want := time.Date(2026, time.August, 4, 23, 0, 30, 0, time.UTC)
	if got := marketsync.NextHourlyRun(now); !got.Equal(want) {
		t.Fatalf("NextHourlyRun(%s) = %s, want %s", now, got, want)
	}
}

func TestSchedulerStartsCatchUpAsynchronouslyAndCancelsItOnShutdown(t *testing.T) {
	runner := &blockingSyncRunner{started: make(chan struct{}), stopped: make(chan struct{})}
	scheduler := marketsync.NewScheduler(runner, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- scheduler.Run(ctx) }()

	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("startup catch-up synchronization did not start")
	}
	cancel()
	select {
	case <-runner.stopped:
	case <-time.After(time.Second):
		t.Fatal("active synchronization was not cancelled")
	}
	if err := <-result; err != nil {
		t.Fatalf("Scheduler.Run() error = %v", err)
	}
}

func TestHourlySchedulerStartsAndCancelsWithoutWaitGroupRace(t *testing.T) {
	daily := &blockingSyncRunner{started: make(chan struct{}), stopped: make(chan struct{}), release: make(chan struct{})}
	hourly := &blockingSyncRunner{started: make(chan struct{}), stopped: make(chan struct{})}
	scheduler := marketsync.NewSchedulerWithHourly(daily, hourly, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- scheduler.Run(ctx) }()

	for _, started := range []chan struct{}{daily.started, hourly.started} {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("scheduled synchronization did not start")
		}
		if started == daily.started {
			close(daily.release)
		}
	}
	cancel()
	for _, stopped := range []chan struct{}{daily.stopped, hourly.stopped} {
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("scheduled synchronization was not cancelled")
		}
	}
	if err := <-result; err != nil {
		t.Fatalf("Scheduler.Run() error = %v", err)
	}
}

func TestSchedulerDiscardsQueuedHourlyRunOnCancellation(t *testing.T) {
	daily := &blockingSyncRunner{started: make(chan struct{}), stopped: make(chan struct{})}
	hourly := &blockingSyncRunner{started: make(chan struct{}), stopped: make(chan struct{})}
	scheduler := marketsync.NewSchedulerWithHourly(daily, hourly, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- scheduler.Run(ctx) }()

	select {
	case <-daily.started:
	case <-time.After(time.Second):
		t.Fatal("daily synchronization did not start")
	}
	cancel()
	select {
	case <-daily.stopped:
	case <-time.After(time.Second):
		t.Fatal("active daily synchronization was not cancelled")
	}
	select {
	case <-hourly.started:
		t.Fatal("queued hourly synchronization started after cancellation")
	case <-time.After(50 * time.Millisecond):
	}
	if err := <-result; err != nil {
		t.Fatalf("Scheduler.Run() error = %v", err)
	}
}

type blockingSyncRunner struct {
	started chan struct{}
	stopped chan struct{}
	release chan struct{}
}

func (runner *blockingSyncRunner) Sync(ctx context.Context) error {
	close(runner.started)
	if runner.release == nil {
		<-ctx.Done()
	} else {
		select {
		case <-runner.release:
		case <-ctx.Done():
		}
	}
	close(runner.stopped)
	return ctx.Err()
}

var _ interface{ Sync(context.Context) error } = (*blockingSyncRunner)(nil)
