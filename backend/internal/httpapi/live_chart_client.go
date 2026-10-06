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

// ChartService validates indicator selections, starts live chart streams,
// and lists the indicator catalog clients draw.
type ChartService interface {
	Validate([]indicator.Selection) error
	NewLiveGroup() *chart.LiveGroup
	Catalog(market.CandleInterval) []chart.CatalogIndicator
}

// chartClient connects the live charts of one connection to its socket. The
// chart streams build frames outside the Binance reader, one goroutine per
// subscribed key; the client numbers and serializes what they deliver.
type chartClient struct {
	socket *liveSocketClient
	group  *chart.LiveGroup
	logger *slog.Logger
	once   sync.Once

	mu      sync.Mutex
	closed  bool
	streams map[kline.Key]chartStream
	// started identifies the streams, so an ended one never delivers after
	// a new stream of its key started.
	started uint64
	// versions number the frames of each key on this connection, also
	// across resubscriptions, so clients ignore late frames.
	versions map[kline.Key]int64
}

// newChartClient serves one connection until Close is called; Wait returns
// once its chart streams have stopped.
func newChartClient(socket *liveSocketClient, charts ChartService, logger *slog.Logger) *chartClient {
	return &chartClient{socket: socket, group: charts.NewLiveGroup(), logger: logger, streams: map[kline.Key]chartStream{}, versions: map[kline.Key]int64{}}
}

type chartStream struct {
	stream *chart.LiveStream
	id     uint64
}

func (c *chartClient) ID() string { return c.socket.ID() }

// Close stops every chart stream and the socket without waiting.
func (c *chartClient) Close() {
	c.once.Do(func() {
		c.stop()
		c.socket.Close()
	})
}

// stop ends every chart stream and rejects new ones.
func (c *chartClient) stop() {
	c.mu.Lock()
	c.closed = true
	streams := c.streams
	c.streams = map[kline.Key]chartStream{}
	c.mu.Unlock()
	for _, entry := range streams {
		entry.stream.Stop()
	}
}

// Wait returns once every chart stream has stopped.
func (c *chartClient) Wait() { c.group.Wait() }

// Enqueue hands a live-service message to the stream of its key. A stream
// exists before its live subscription starts, so messages for keys without
// one are late deliveries of an ended subscription and are dropped.
func (c *chartClient) Enqueue(message marketlive.Message) bool {
	c.mu.Lock()
	closed, entry := c.closed, c.streams[message.Key]
	c.mu.Unlock()
	if closed {
		return false
	}
	if entry.stream == nil {
		return true
	}
	if !entry.stream.Enqueue(message) {
		c.Close()
		return false
	}
	return true
}

// setRange sets the chart range of key. A new key starts a stream whose first
// snapshot is driven by the live subscription snapshot; an existing key asks
// for a rebuilt snapshot of the new range.
func (c *chartClient) setRange(ctx context.Context, key kline.Key, limit int, configs []indicator.Selection) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	entry := c.streams[key]
	if entry.stream == nil {
		c.started++
		id := c.started
		stream := c.group.Start(ctx, key.Symbol, key.Interval, limit, configs, func(output chart.Output) bool {
			return c.deliver(key, id, output)
		})
		c.streams[key] = chartStream{stream: stream, id: id}
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	if !entry.stream.SetRange(limit, configs) {
		c.Close()
	}
}

// forget stops the stream of key after an unsubscribe or failed subscribe.
func (c *chartClient) forget(key kline.Key) {
	c.mu.Lock()
	entry, ok := c.streams[key]
	delete(c.streams, key)
	c.mu.Unlock()
	if ok {
		entry.stream.Stop()
	}
}

// deliver sends output only while stream id still owns key, so nothing of an
// ended subscription follows its unsubscribed message. It reports whether the
// stream should go on.
func (c *chartClient) deliver(key kline.Key, id uint64, output chart.Output) bool {
	var wire LiveCandleServerMessage
	switch {
	case output.Message != nil:
		wire = liveWireMessage(*output.Message)
	case output.Err != nil:
		wire = keyErrorMessage(key, "unavailable", "Chart history is unavailable")
	default:
		// A range change carries no stream state; report the last known one.
		wire = liveWireMessage(marketlive.Message{Key: key, Freshness: output.Freshness})
		wire.Type = Update
		if output.Frame.Snapshot {
			wire.Type = Snapshot
		}
		wire.Chart = chartWirePage(output.Frame.Page, key.Interval, output.Frame.From)
	}
	c.mu.Lock()
	if entry, ok := c.streams[key]; !ok || entry.id != id {
		c.mu.Unlock()
		return false
	}
	if output.Frame != nil {
		c.versions[key]++
		version := c.versions[key]
		wire.Version = &version
	}
	sent := c.socket.enqueueWire(wire)
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
