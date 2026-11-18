package live

import (
	"context"
	"time"

	"crypto-scanner/internal/market/kline"
)

// wakeRequestsLocked wakes Run to start the requested seeds; it never blocks.
func (service *Service) wakeRequestsLocked() {
	select {
	case service.requests <- struct{}{}:
	default:
	}
}

// startRequested starts the seeds requested outside Run's goroutine; ctx
// bounds them.
func (service *Service) startRequested(ctx context.Context) {
	service.mu.Lock()
	defer service.mu.Unlock()
	for key, state := range service.states {
		if state.seedRequested {
			state.seedRequested = false
			service.workers.Go(func() { service.seed(ctx, key) })
		}
	}
}

// seed publishes the forming candle of key read from the exchange, so a new
// subscription shows it before the stream's first event: the exchange sends
// one only when a trade changes the candle.
func (service *Service) seed(ctx context.Context, key kline.Key) {
	ctx, cancel := context.WithTimeout(ctx, seedTimeout)
	defer cancel()
	candle, err := service.current.CurrentCandle(ctx, key.Symbol, key.Interval)
	if err != nil {
		service.logger.InfoContext(ctx, "live candle seed failed", "module", "market_live", "operation", "seed", "symbol", key.Symbol, "interval", key.Interval, "error", err)
		return
	}
	service.mu.Lock()
	state := service.states[key]
	// A closed candle is history, and the stream may already be ahead.
	if state == nil || candle.OpenTime.Before(key.Interval.OpenTime(time.Now())) || !latestOpen(state.candles).Before(candle.OpenTime) {
		service.mu.Unlock()
		return
	}
	incoming := CandleState{Candle: candle}
	state.candles[candle.OpenTime] = incoming
	trimCandles(state.candles)
	dropped := service.publishLocked(nil, state.clients, Message{Kind: KindUpdate, Key: key, Candle: &incoming, Freshness: state.freshness()})
	service.mu.Unlock()
	service.closeClients(dropped)
}
