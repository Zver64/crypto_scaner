package httpapi

import (
	"context"
	"sync"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"
)

// ChartService validates indicator selections and starts live chart sessions.
type ChartService interface {
	Validate([]indicator.Selection) error
	NewLiveSession(string, market.CandleInterval) *chart.LiveSession
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

// chartClient serializes per-connection chart calculations outside the Binance
// reader. Its bounded mailbox cannot exert backpressure on the market stream.
type chartClient struct {
	socket *liveSocketClient
	charts ChartService
	events chan chartEvent
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	mu       sync.Mutex
	ranges   map[kline.Key]chartRange
	versions map[kline.Key]int64
}

// newChartClient serves one connection until ctx ends or Close is called.
func newChartClient(ctx context.Context, socket *liveSocketClient, charts ChartService) *chartClient {
	ctx, cancel := context.WithCancel(ctx)
	c := &chartClient{socket: socket, charts: charts, events: make(chan chartEvent, 32), ctx: ctx, cancel: cancel, ranges: map[kline.Key]chartRange{}, versions: map[kline.Key]int64{}}
	go c.run(ctx)
	return c
}

func (c *chartClient) ID() string { return c.socket.ID() }
func (c *chartClient) Close()     { c.once.Do(func() { c.cancel(); c.socket.Close() }) }
func (c *chartClient) Enqueue(message marketlive.Message) bool {
	return c.enqueue(chartEvent{message: message})
}

func (c *chartClient) enqueue(event chartEvent) bool {
	select {
	case <-c.ctx.Done():
		return false
	case c.events <- event:
		return true
	default:
		c.Close()
		return false
	}
}

// setRange changes the chart range of key and asks for a rebuilt snapshot.
func (c *chartClient) setRange(key kline.Key, limit int, configs []indicator.Selection) {
	c.mu.Lock()
	c.ranges[key] = chartRange{limit: limit, configs: configs}
	c.mu.Unlock()
	c.enqueue(chartEvent{message: marketlive.Message{Key: key}, rangeChanged: true})
}

func (c *chartClient) forget(key kline.Key) {
	c.mu.Lock()
	delete(c.ranges, key)
	c.mu.Unlock()
}

func (c *chartClient) run(ctx context.Context) {
	sessions := map[kline.Key]*chart.LiveSession{}
	freshness := map[kline.Key]marketlive.Freshness{}
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-c.events:
			message := event.message
			if message.Freshness != "" {
				freshness[message.Key] = message.Freshness
			}
			session := sessions[message.Key]
			if session == nil {
				session = c.charts.NewLiveSession(message.Key.Symbol, message.Key.Interval)
				sessions[message.Key] = session
			}
			trigger := chart.TriggerRange
			if !event.rangeChanged {
				var ok bool
				if trigger, ok = session.Apply(message); !ok {
					if !c.socket.Enqueue(message) {
						c.Close()
						return
					}
					continue
				}
			}
			c.mu.Lock()
			current, subscribed := c.ranges[message.Key]
			c.mu.Unlock()
			if !subscribed {
				session.Forget()
				continue
			}
			frame, ok, err := session.Next(ctx, trigger, current.limit, current.configs)
			if err != nil {
				c.socket.enqueueKeyError(message.Key, "unavailable", "Chart history is unavailable")
				continue
			}
			if !ok {
				continue
			}
			c.mu.Lock()
			if c.ranges[message.Key].limit != current.limit {
				session.DropRange()
				c.mu.Unlock()
				continue
			}
			c.versions[message.Key]++
			version := c.versions[message.Key]
			c.mu.Unlock()
			// A range change carries no stream state; report the last known one.
			message.Freshness = freshness[message.Key]
			wire := liveWireMessage(message)
			wire.Type = Update
			if frame.Snapshot {
				wire.Type = Snapshot
			}
			wire.Chart = chartWirePage(frame.Page, message.Key.Interval, frame.From)
			wire.Version = &version
			if !c.socket.enqueueWire(wire) {
				c.Close()
				return
			}
		}
	}
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
