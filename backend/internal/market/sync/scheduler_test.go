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

type blockingSyncRunner struct {
	started chan struct{}
	stopped chan struct{}
}

func (runner *blockingSyncRunner) Sync(ctx context.Context) error {
	close(runner.started)
	<-ctx.Done()
	close(runner.stopped)
	return ctx.Err()
}

var _ interface{ Sync(context.Context) error } = (*blockingSyncRunner)(nil)
