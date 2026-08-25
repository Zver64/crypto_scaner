package sync

import (
	"context"
	"testing"
)

func TestJobQueueCoalescesPendingRunsPerProfile(t *testing.T) {
	queue := newJobQueue()
	runner := queueTestRunner{}
	queue.enqueue("hourly", runner)
	queue.enqueue("hourly", runner)
	queue.enqueue("daily", runner)
	queue.enqueue("daily", runner)

	seen := map[string]bool{}
	for range 2 {
		job := <-queue.jobs
		if seen[job.profile] {
			t.Fatalf("profile %q was queued more than once", job.profile)
		}
		seen[job.profile] = true
		queue.complete(job.profile)
	}
	if !seen["daily"] || !seen["hourly"] {
		t.Fatalf("queued profiles = %#v, want daily and hourly", seen)
	}
}

type queueTestRunner struct{}

func (queueTestRunner) Sync(context.Context) error { return nil }
