package live

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
)

// Recovery of a gap the stream missed has no timer and never polls: one read
// takes what storage already holds, and HistoryChanged delivers what
// synchronization commits later. The key reports recovering until the gap is
// covered.

// startReadLocked starts a read of the stored candles covering the gap of
// key when one is wanted and none runs. It runs on Run's goroutine, whose ctx
// bounds the read; service.mu must be held.
func (service *Service) startReadLocked(ctx context.Context, key kline.Key, state *keyState) {
	if state.gap.empty() || !state.readWanted || state.reading {
		return
	}
	state.reading = true
	service.workers.Go(func() { service.read(ctx, key, state) })
}

// read reads the stored candles covering the gap of key once per request; a
// gap grown meanwhile is read again. A failed read is retried by the key's
// next stream event.
func (service *Service) read(ctx context.Context, key kline.Key, state *keyState) {
	for {
		service.mu.Lock()
		// A released key may have a new state with its own read.
		if service.states[key] != state {
			service.mu.Unlock()
			return
		}
		if state.gap.empty() || !state.readWanted {
			state.reading = false
			service.mu.Unlock()
			return
		}
		state.readWanted = false
		instrumentID, through := state.instrument.ID, state.gap.through
		service.mu.Unlock()

		candles, err := service.readStored(ctx, instrumentID, key.Interval, through)

		service.mu.Lock()
		if service.states[key] != state {
			service.mu.Unlock()
			return
		}
		if err != nil {
			state.readWanted = true
			state.reading = false
			service.mu.Unlock()
			if ctx.Err() == nil {
				service.readFailure.Warn(ctx, "live candle history read failed", "module", "market_live", "operation", "recover", "symbol", key.Symbol, "interval", key.Interval, "error", err)
			}
			return
		}
		state.restore(key.Interval, candles)
		dropped := service.publishLocked(nil, state.clients, snapshotMessage(key, state))
		service.mu.Unlock()
		service.closeClients(dropped)
		service.readFailure.Recover(ctx, "live candle history read recovered", "module", "market_live", "operation", "recover")
	}
}

// readStored reads the newest stored candles up to through, as many as the
// buffer keeps. The process-wide limiter keeps many recovering keys from
// overwhelming PostgreSQL after a mass reconnect.
func (service *Service) readStored(ctx context.Context, instrumentID int64, interval market.CandleInterval, through time.Time) ([]market.Candle, error) {
	if err := service.readLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	before := interval.NextOpenTime(through)
	page, err := service.store.ListCandlePage(ctx, instrumentID, interval, &before, maxRetainedCandles)
	if err != nil {
		return nil, fmt.Errorf("list stored candles: %w", err)
	}
	return page.Candles, nil
}
