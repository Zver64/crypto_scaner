package chart

import (
	"context"
	"sync"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"
)

// maxPendingEvents bounds the events waiting for one live stream. Workers
// drain them all at once, so only a stuck client reaches it.
const maxPendingEvents = 64

// Output is one delivery of a live stream: a frame, a live-service message
// that carries no candle state, or a history failure the client must see.
type Output struct {
	Frame *Frame
	// Freshness is the last known stream freshness, reported with frames.
	Freshness marketlive.Freshness
	Message   *marketlive.Message
	Err       error
}

// Deliver hands an output to the client of a stream. It must not block and
// returns false when the stream should stop, for example because the client
// is gone or no longer owns the stream.
type Deliver func(Output) bool

// LiveGroup runs the live streams of one client and waits for them.
type LiveGroup struct {
	service *Service
	workers sync.WaitGroup
}

// NewLiveGroup starts an empty group of live streams.
func (service *Service) NewLiveGroup() *LiveGroup { return &LiveGroup{service: service} }

// Wait returns once every stream of the group has stopped. Streams stop when
// their context ends, when Stop is called, or when delivery fails.
func (group *LiveGroup) Wait() { group.workers.Wait() }

type streamEvent struct {
	message      marketlive.Message
	rangeChanged bool
}

// LiveStream builds the chart of one symbol and interval on its own goroutine.
// It merges the events queued during a build and builds a single frame for the
// strongest trigger among them, so a slow build never holds up the market
// stream.
type LiveStream struct {
	symbol     string
	interval   market.CandleInterval
	deliver    Deliver
	wake       chan struct{}
	cancel     context.CancelFunc
	subscribed time.Time

	mu      sync.Mutex
	limit   int
	configs []indicator.Selection
	pending []streamEvent
}

// Start runs a stream of the range until ctx ends or Stop is called. Its first
// snapshot is driven by the live subscription snapshot.
func (group *LiveGroup) Start(ctx context.Context, symbol string, interval market.CandleInterval, limit int, configs []indicator.Selection, deliver Deliver) *LiveStream {
	ctx, cancel := context.WithCancel(ctx)
	stream := &LiveStream{
		symbol: symbol, interval: interval, deliver: deliver, wake: make(chan struct{}, 1), cancel: cancel, subscribed: time.Now(),
		limit: limit, configs: configs,
	}
	group.workers.Go(func() {
		defer cancel()
		stream.run(ctx, group.service.NewLiveSession(symbol, interval), group.service)
	})
	return stream
}

// Stop ends the stream and cancels a build in progress without waiting.
func (stream *LiveStream) Stop() { stream.cancel() }

// Enqueue queues a live-service message. It returns false when the stream
// has too many pending events, so the client cannot keep up.
func (stream *LiveStream) Enqueue(message marketlive.Message) bool {
	return stream.enqueue(streamEvent{message: message})
}

// SetRange changes the range and asks for a rebuilt snapshot of it.
func (stream *LiveStream) SetRange(limit int, configs []indicator.Selection) bool {
	stream.mu.Lock()
	stream.limit, stream.configs = limit, configs
	stream.mu.Unlock()
	return stream.enqueue(streamEvent{message: marketlive.Message{Key: stream.key()}, rangeChanged: true})
}

func (stream *LiveStream) key() kline.Key {
	return kline.Key{Symbol: stream.symbol, Interval: stream.interval}
}

func (stream *LiveStream) enqueue(event streamEvent) bool {
	stream.mu.Lock()
	if len(stream.pending) >= maxPendingEvents {
		stream.mu.Unlock()
		return false
	}
	stream.pending = append(stream.pending, event)
	stream.mu.Unlock()
	select {
	case stream.wake <- struct{}{}:
	default:
	}
	return true
}

func (stream *LiveStream) run(ctx context.Context, session *LiveSession, service *Service) {
	var freshness marketlive.Freshness
	delivered := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.wake:
		}
		stream.mu.Lock()
		events := stream.pending
		stream.pending = nil
		limit, configs := stream.limit, stream.configs
		stream.mu.Unlock()
		trigger, triggered := TriggerTrade, false
		for _, event := range events {
			if event.message.Freshness != "" {
				freshness = event.message.Freshness
			}
			next := TriggerRange
			if !event.rangeChanged {
				var ok bool
				if next, ok = session.Apply(event.message); !ok {
					if !stream.deliver(Output{Message: &event.message}) {
						return
					}
					continue
				}
			}
			trigger, triggered = max(trigger, next), true
		}
		if !triggered {
			continue
		}
		started := time.Now()
		frame, ok, err := session.Next(ctx, trigger, limit, configs)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if !stream.deliver(Output{Err: err}) {
				return
			}
			continue
		}
		if !ok {
			continue
		}
		// A queued range change rebuilds the snapshot, so a frame of the old
		// range is never delivered after it was set.
		stream.mu.Lock()
		if stream.limit != limit {
			session.DropRange()
			stream.mu.Unlock()
			continue
		}
		sent := stream.deliver(Output{Frame: &frame, Freshness: freshness})
		stream.mu.Unlock()
		if !sent {
			return
		}
		attributes := []any{"operation", "chart_frame", "symbol", stream.symbol, "interval", stream.interval,
			"limit", limit, "snapshot", frame.Snapshot, "events", len(events), "build_duration", time.Since(started)}
		switch {
		case frame.Snapshot && !delivered:
			delivered = true
			service.logger.InfoContext(ctx, "chart first snapshot sent", append(attributes, "since_subscribe", time.Since(stream.subscribed))...)
		case trigger == TriggerRange:
			service.logger.InfoContext(ctx, "chart range snapshot sent", attributes...)
		default:
			service.logger.DebugContext(ctx, "chart frame sent", attributes...)
		}
	}
}
