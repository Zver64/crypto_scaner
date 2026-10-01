package httpapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"
)

// ChartService validates indicator selections, starts live chart sessions,
// and lists the indicator catalog clients draw.
type ChartService interface {
	Validate([]indicator.Selection) error
	NewLiveSession(string, market.CandleInterval) *chart.LiveSession
	Catalog(market.CandleInterval) []chart.CatalogIndicator
}

type chartRange struct {
	limit   int
	configs []indicator.Selection
}

// chartEvent is either a live-service message or a client range change.
type chartEvent struct {
	message      marketlive.Message
	rangeChanged bool
}

// maxPendingChartEvents bounds the events waiting for one chart. Workers
// drain them all at once, so only a stuck client reaches it.
const maxPendingChartEvents = 64

// chartClient calculates the charts of one connection outside the Binance
// reader. Each subscribed key has its own worker, so charts build in parallel,
// and its mailbox cannot exert backpressure on the market stream.
type chartClient struct {
	socket *liveSocketClient
	charts ChartService
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	mu       sync.Mutex
	streams  map[kline.Key]*chartStream
	versions map[kline.Key]int64
}

// chartStream is the worker state of one subscribed key. Fields other than
// key, wake, ctx, and cancel are guarded by chartClient.mu.
type chartStream struct {
	key  kline.Key
	wake chan struct{}
	// ctx ends with the subscription and cancels a build in progress.
	ctx    context.Context
	cancel context.CancelFunc

	chartRange   chartRange
	pending      []chartEvent
	subscribedAt time.Time
}

// newChartClient serves one connection until ctx ends or Close is called.
func newChartClient(ctx context.Context, socket *liveSocketClient, charts ChartService, logger *slog.Logger) *chartClient {
	ctx, cancel := context.WithCancel(ctx)
	return &chartClient{socket: socket, charts: charts, logger: logger, ctx: ctx, cancel: cancel, streams: map[kline.Key]*chartStream{}, versions: map[kline.Key]int64{}}
}

func (c *chartClient) ID() string { return c.socket.ID() }
func (c *chartClient) Close()     { c.once.Do(func() { c.cancel(); c.socket.Close() }) }

// Enqueue hands a live-service message to the worker of its key. A worker
// exists before its live subscription starts, so messages for keys without
// one are late deliveries of an ended subscription and are dropped.
func (c *chartClient) Enqueue(message marketlive.Message) bool {
	if c.ctx.Err() != nil {
		return false
	}
	c.mu.Lock()
	stream := c.streams[message.Key]
	c.mu.Unlock()
	if stream == nil {
		return true
	}
	return c.enqueue(stream, chartEvent{message: message})
}

func (c *chartClient) enqueue(stream *chartStream, event chartEvent) bool {
	c.mu.Lock()
	if len(stream.pending) >= maxPendingChartEvents {
		c.mu.Unlock()
		c.Close()
		return false
	}
	stream.pending = append(stream.pending, event)
	c.mu.Unlock()
	select {
	case stream.wake <- struct{}{}:
	default:
	}
	return true
}

// setRange sets the chart range of key. A new key starts a worker whose first
// snapshot is driven by the live subscription snapshot; an existing key asks
// for a rebuilt snapshot of the new range.
func (c *chartClient) setRange(key kline.Key, limit int, configs []indicator.Selection) {
	c.mu.Lock()
	stream := c.streams[key]
	if stream == nil {
		ctx, cancel := context.WithCancel(c.ctx)
		stream = &chartStream{key: key, wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel, chartRange: chartRange{limit: limit, configs: configs}, subscribedAt: time.Now()}
		c.streams[key] = stream
		c.mu.Unlock()
		go c.run(stream)
		return
	}
	stream.chartRange = chartRange{limit: limit, configs: configs}
	c.mu.Unlock()
	c.enqueue(stream, chartEvent{message: marketlive.Message{Key: key}, rangeChanged: true})
}

// forget stops the worker of key after an unsubscribe or failed subscribe.
func (c *chartClient) forget(key kline.Key) {
	c.mu.Lock()
	stream := c.streams[key]
	delete(c.streams, key)
	c.mu.Unlock()
	if stream != nil {
		stream.cancel()
	}
}

// run serializes the chart of one key. It applies every queued event and then
// builds a single frame for the strongest trigger among them.
func (c *chartClient) run(stream *chartStream) {
	ctx, key := stream.ctx, stream.key
	session := c.charts.NewLiveSession(key.Symbol, key.Interval)
	var freshness marketlive.Freshness
	delivered := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.wake:
		}
		c.mu.Lock()
		events := stream.pending
		stream.pending = nil
		current := stream.chartRange
		c.mu.Unlock()
		trigger, triggered := chart.TriggerTrade, false
		for _, event := range events {
			if event.message.Freshness != "" {
				freshness = event.message.Freshness
			}
			next := chart.TriggerRange
			if !event.rangeChanged {
				var ok bool
				if next, ok = session.Apply(event.message); !ok {
					if !c.deliver(stream, liveWireMessage(event.message)) {
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
		frame, ok, err := session.Next(ctx, trigger, current.limit, current.configs)
		if err != nil {
			if !c.deliver(stream, keyErrorMessage(key, "unavailable", "Chart history is unavailable")) {
				return
			}
			continue
		}
		if !ok {
			continue
		}
		// A range change carries no stream state; report the last known one.
		wire := liveWireMessage(marketlive.Message{Key: key, Freshness: freshness})
		wire.Type = Update
		if frame.Snapshot {
			wire.Type = Snapshot
		}
		wire.Chart = chartWirePage(frame.Page, key.Interval, frame.From)
		c.mu.Lock()
		if c.streams[key] != stream {
			// An unsubscribe or a new subscription of key owns its messages now.
			c.mu.Unlock()
			return
		}
		if stream.chartRange.limit != current.limit {
			// A queued range change rebuilds the snapshot.
			session.DropRange()
			c.mu.Unlock()
			continue
		}
		c.versions[key]++
		version := c.versions[key]
		wire.Version = &version
		sent := c.socket.enqueueWire(wire)
		c.mu.Unlock()
		if !sent {
			c.Close()
			return
		}
		attributes := []any{"module", "httpapi", "operation", "chart_frame", "symbol", key.Symbol, "interval", key.Interval,
			"limit", current.limit, "snapshot", frame.Snapshot, "events", len(events), "build_duration", time.Since(started)}
		switch {
		case frame.Snapshot && !delivered:
			delivered = true
			c.logger.InfoContext(ctx, "chart first snapshot sent", append(attributes, "since_subscribe", time.Since(stream.subscribedAt))...)
		case trigger == chart.TriggerRange:
			c.logger.InfoContext(ctx, "chart range snapshot sent", attributes...)
		default:
			c.logger.DebugContext(ctx, "chart frame sent", attributes...)
		}
	}
}

// deliver sends message only while stream still owns its key, so nothing of an
// ended subscription follows its unsubscribed message. It reports whether the
// worker should go on.
func (c *chartClient) deliver(stream *chartStream, message LiveCandleServerMessage) bool {
	c.mu.Lock()
	if c.streams[stream.key] != stream {
		c.mu.Unlock()
		return false
	}
	sent := c.socket.enqueueWire(message)
	c.mu.Unlock()
	if !sent {
		c.Close()
	}
	return sent
}

// chartWirePage serializes candles and indicator points opened at or after from.
func chartWirePage(page chart.Page, interval market.CandleInterval, from time.Time) *ChartPageResponse {
	candles := make([]Candle, 0, len(page.Candles))
	for _, v := range page.Candles {
		if !v.OpenTime.Before(from) {
			candles = append(candles, candleResponse(v))
		}
	}
	results := make([]ChartIndicatorResult, len(page.Indicators))
	for i, v := range page.Indicators {
		series := make([]IndicatorSeries, len(v.Series))
		for j, s := range v.Series {
			points := make([]IndicatorPoint, 0, len(s.Points))
			for _, p := range s.Points {
				if !p.Time.Before(from) {
					points = append(points, IndicatorPoint{Time: p.Time, Value: p.Value})
				}
			}
			series[j] = IndicatorSeries{Name: s.Name, Points: points}
		}
		results[i] = ChartIndicatorResult{Type: string(v.Type), Parameters: map[string]interface{}(v.Parameters), Series: series}
	}
	return &ChartPageResponse{Symbol: page.Symbol, Interval: CandleInterval(interval), Candles: candles, Indicators: results, HasMore: page.HasMore, NextBefore: page.NextBefore}
}
