package httpapi

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/market"
	marketlive "crypto-scanner/internal/market/live"
)

const (
	maxChartRange = 5000
	// A failed chart build is retried on trades no more often than this.
	chartRetryDelay = 10 * time.Second
)

// chartClient serializes per-connection chart calculations outside the Binance
// reader. Its bounded mailbox cannot exert backpressure on the market stream.
type chartClient struct {
	socket   *liveSocketClient
	charts   ChartService
	logger   *slog.Logger
	events   chan marketlive.Message
	done     chan struct{}
	once     sync.Once
	mu       sync.Mutex
	limits   map[binance.KlineKey]int
	configs  map[binance.KlineKey][]chart.IndicatorConfig
	versions map[binance.KlineKey]int64
}

// ChartService calculates a closed-candle range and its private extension with
// the current candle using the same indicator engine.
type ChartService interface {
	Build(context.Context, chart.Request) (chart.Page, error)
	Validate([]chart.IndicatorConfig) error
	Extend(chart.Page, market.CandleInterval, *market.Candle, []chart.IndicatorConfig) (chart.Page, error)
}

func newChartClient(socket *liveSocketClient, charts ChartService, logger *slog.Logger) *chartClient {
	c := &chartClient{socket: socket, charts: charts, logger: logger, events: make(chan marketlive.Message, 32), done: make(chan struct{}), limits: map[binance.KlineKey]int{}, configs: map[binance.KlineKey][]chart.IndicatorConfig{}, versions: map[binance.KlineKey]int64{}}
	go c.run()
	return c
}
func (c *chartClient) ID() string { return c.socket.ID() }
func (c *chartClient) Close()     { c.once.Do(func() { close(c.done); c.socket.Close() }) }
func (c *chartClient) Enqueue(message marketlive.Message) bool {
	select {
	case <-c.done:
		return false
	case c.events <- message:
		return true
	default:
		c.Close()
		return false
	}
}
func (c *chartClient) setRange(key binance.KlineKey, limit int, configs []chart.IndicatorConfig) {
	c.mu.Lock()
	c.limits[key] = limit
	c.configs[key] = configs
	c.mu.Unlock()
}
func (c *chartClient) forget(key binance.KlineKey) {
	c.mu.Lock()
	delete(c.limits, key)
	delete(c.configs, key)
	c.mu.Unlock()
}
func (c *chartClient) run() {
	states := map[binance.KlineKey]map[time.Time]marketlive.CandleState{}
	closed := map[binance.KlineKey]chart.Page{}
	freshness := map[binance.KlineKey]marketlive.Freshness{}
	retryAt := map[binance.KlineKey]time.Time{}
	for {
		select {
		case <-c.done:
			return
		case message := <-c.events:
			if message.Freshness != "" {
				freshness[message.Key] = message.Freshness
			}
			if message.Kind == "snapshot" {
				states[message.Key] = map[time.Time]marketlive.CandleState{}
				for _, s := range message.Candles {
					states[message.Key][s.Candle.OpenTime] = s
				}
			}
			if message.Kind == "update" && message.Candle != nil {
				if states[message.Key] == nil {
					states[message.Key] = map[time.Time]marketlive.CandleState{}
				}
				old := states[message.Key][message.Candle.Candle.OpenTime]
				if !old.Final || message.Candle.Final {
					states[message.Key][message.Candle.Candle.OpenTime] = *message.Candle
				}
			}
			if message.Kind != "update" && message.Kind != "snapshot" && message.Kind != "refresh" {
				if !c.socket.Enqueue(message) {
					c.Close()
					return
				}
				continue
			}
			c.mu.Lock()
			limit := c.limits[message.Key]
			configs := c.configs[message.Key]
			c.mu.Unlock()
			if limit == 0 {
				delete(closed, message.Key)
				delete(retryAt, message.Key)
				continue
			}
			page, cached := closed[message.Key]
			// Only a trade on the current candle leaves the closed range intact;
			// everything else rebuilds it and is delivered as a full snapshot.
			trade := message.Kind == "update" && message.Candle != nil && !message.Candle.Final
			rebuild := !cached || !trade
			if !cached && trade && time.Now().Before(retryAt[message.Key]) {
				continue
			}
			if rebuild {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				effectiveLimit := limit
				if cached && limit > defaultCandlePageSize && len(page.Candles) > effectiveLimit {
					effectiveLimit = min(len(page.Candles), maxChartRange)
				}
				loaded, err := c.charts.Build(ctx, chart.Request{Symbol: message.Key.Symbol, Interval: message.Key.Interval, Limit: effectiveLimit, Indicators: configs})
				if err == nil && cached && limit > defaultCandlePageSize && len(page.Candles) > 0 && len(loaded.Candles) > 0 && loaded.HasMore {
					oldest := page.Candles[0].OpenTime
					cursor := oldest
					for cursor.Before(loaded.Candles[0].OpenTime) && effectiveLimit < maxChartRange {
						effectiveLimit++
						cursor = message.Key.Interval.NextOpenTime(cursor)
					}
					if cursor.After(oldest) {
						loaded, err = c.charts.Build(ctx, chart.Request{Symbol: message.Key.Symbol, Interval: message.Key.Interval, Limit: effectiveLimit, Indicators: configs})
					}
				}
				cancel()
				if err != nil {
					failing := !retryAt[message.Key].IsZero()
					retryAt[message.Key] = time.Now().Add(chartRetryDelay)
					if !failing {
						c.logger.Warn("chart history build failed", "module", "httpapi_live", "symbol", message.Key.Symbol, "interval", message.Key.Interval, "error", err)
					}
					if !cached || message.Kind == "refresh" {
						// Report once per failure, and always for a range request.
						if !failing || message.Kind == "refresh" {
							c.socket.enqueueKeyError(message.Key, "unavailable", "Chart history is unavailable")
						}
						continue
					}
					// Keep the last closed range; final WS candles below still
					// advance it, and the result is delivered as a snapshot.
					loaded = page
				} else {
					delete(retryAt, message.Key)
				}
				page = loaded
			}
			live := states[message.Key]
			keys := make([]time.Time, 0, len(live))
			for key := range live {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
			// Final WS candles take precedence over stale REST history. Until the
			// previous candle is confirmed final, a new trade still updates its price.
			candles := append([]market.Candle(nil), page.Candles...)
			var pending *market.Candle
			for _, key := range keys {
				state := live[key]
				value := state.Candle
				if !state.Final {
					if len(candles) > 0 && !key.After(candles[len(candles)-1].OpenTime) {
						continue
					}
					if pending != nil && value.OpenTime.After(pending.OpenTime) {
						// Do not advance the open time until the previous
						// candle's final event has actually arrived; keep the
						// candle valid as the newer price leaves its range.
						pending.Close = value.Close
						pending.High = max(pending.High, value.High)
						pending.Low = min(pending.Low, value.Low)
					} else if pending == nil {
						pending = &value
					}
					continue
				}
				// A persisted close takes precedence over a delayed WS final.
				// A new final absent from history is appended below.
				if len(candles) == 0 || message.Key.Interval.NextOpenTime(candles[len(candles)-1].OpenTime).Equal(key) {
					candles = append(candles, value)
				}
				if pending != nil && pending.OpenTime.Equal(key) {
					pending = nil
				}
			}
			// The initial range stays fixed; an extended one keeps its oldest
			// candle but never exceeds the maximum chart range.
			keep := maxChartRange
			if limit == defaultCandlePageSize {
				keep = limit
			}
			if len(candles) > keep {
				candles = candles[len(candles)-keep:]
				page.HasMore = true
			}
			page.Candles = candles
			if page.HasMore && len(candles) > 0 {
				oldest := candles[0].OpenTime.UTC()
				page.NextBefore = &oldest
			}
			if pending != nil && len(candles) > 0 {
				last := candles[len(candles)-1].OpenTime
				if pending.OpenTime.Before(last) || pending.OpenTime.After(message.Key.Interval.NextOpenTime(last)) {
					pending = nil
				}
			}
			closed[message.Key] = page
			if !rebuild && pending == nil {
				continue
			}
			page, err := c.charts.Extend(page, message.Key.Interval, pending, configs)
			if err != nil {
				continue
			}
			c.mu.Lock()
			if c.limits[message.Key] != limit {
				delete(closed, message.Key)
				c.mu.Unlock()
				continue
			}
			c.versions[message.Key]++
			version := c.versions[message.Key]
			c.mu.Unlock()
			// A refresh carries no stream state; report the last known one.
			message.Freshness = freshness[message.Key]
			wire := liveWireMessage(message)
			if rebuild {
				wire.Type = Snapshot
				wire.Chart = chartWirePage(page, message.Key.Interval, time.Time{})
			} else {
				// Closed points are unchanged, so an update carries only the tail.
				wire.Type = Update
				wire.Chart = chartWirePage(page, message.Key.Interval, pending.OpenTime)
			}
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
