// Package delivery runs the workers that deliver queued messages, such as
// alerts, and still deliver those queued at shutdown.
package delivery

import (
	"context"
	"sync"
	"time"
)

// drainTimeout bounds the delivery of the messages queued at shutdown.
const drainTimeout = 10 * time.Second

// Start delivers the items of queue with workers goroutines until ctx ends,
// and then the items still queued. Deliveries get a context that outlives
// ctx, so those of the final pass are not aborted at once. The returned wait,
// called once ctx has ended, returns when the workers have stopped,
// cancelling their deliveries once drainTimeout has passed.
func Start[T any](ctx context.Context, workers int, queue <-chan T, deliver func(context.Context, T)) (wait func()) {
	deliverCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	var running sync.WaitGroup
	for range workers {
		running.Go(func() { work(ctx, deliverCtx, queue, deliver) })
	}
	return func() {
		defer stop()
		done := make(chan struct{})
		go func() {
			running.Wait()
			close(done)
		}()
		deadline := time.NewTimer(drainTimeout)
		defer deadline.Stop()
		select {
		case <-done:
		case <-deadline.C:
			stop()
			<-done
		}
	}
}

// work delivers items of queue with deliverCtx until ctx ends and then those
// still queued, while deliverCtx lasts.
func work[T any](ctx, deliverCtx context.Context, queue <-chan T, deliver func(context.Context, T)) {
	for {
		select {
		case <-ctx.Done():
			for deliverCtx.Err() == nil {
				select {
				case item := <-queue:
					deliver(deliverCtx, item)
				default:
					return
				}
			}
			return
		case item := <-queue:
			deliver(deliverCtx, item)
		}
	}
}
