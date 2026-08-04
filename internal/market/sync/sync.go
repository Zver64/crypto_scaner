package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"crypto-scanner/internal/market"
)

// Exchange is the discovery boundary required by instrument synchronization.
type Exchange interface {
	ListInstruments(context.Context) ([]market.Instrument, error)
	ListClosedCandles(context.Context, market.CandleRequest) ([]market.Candle, error)
}

// Store is the persistence boundary required by instrument synchronization.
type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	SaveSyncState(context.Context, market.SyncState) error
	ApplyInstrumentSnapshot(context.Context, []market.Instrument) error
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
	ListLatestCandles(context.Context, int64, int) ([]market.Candle, error)
	UpsertCandles(context.Context, []market.Candle) error
}

// MVPProfile returns the code-owned Binance Spot synchronization profile.
func MVPProfile() market.SyncProfile {
	return market.SyncProfile{
		Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1d", TimeZone: "UTC",
	}
}

// Synchronizer coordinates instrument discovery and initial candle backfill.
type Synchronizer struct {
	exchange Exchange
	store    Store
	logger   *slog.Logger
}

// New creates a market synchronizer that discards structured run logs.
func New(exchange Exchange, store Store) *Synchronizer {
	return NewWithLogger(exchange, store, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

// NewWithLogger creates a market synchronizer that emits per-run totals.
func NewWithLogger(exchange Exchange, store Store, logger *slog.Logger) *Synchronizer {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	return &Synchronizer{exchange: exchange, store: store, logger: logger}
}

// Sync applies the catalog and backfills active instruments without history.
func (synchronizer *Synchronizer) Sync(ctx context.Context) (syncErr error) {
	profile := MVPProfile()
	operationStartedAt := time.Now()
	stats := runStats{}
	defer func() {
		outcome := "succeeded"
		if syncErr != nil {
			outcome = "failed"
		}
		synchronizer.logger.InfoContext(ctx, "market synchronization completed",
			"module", "market_sync", "operation", "sync", "profile", profile.Key(),
			"duration", time.Since(operationStartedAt), "outcome", outcome,
			"instruments_total", stats.instrumentsTotal, "instruments_succeeded", stats.instrumentsSucceeded,
			"instruments_failed", stats.instrumentsFailed, "candle_rows_written", stats.candleRowsWritten,
		)
	}()
	state, err := synchronizer.store.GetSyncState(ctx, profile)
	if err != nil {
		return fmt.Errorf("load synchronization state: %w", err)
	}
	startedAt := time.Now().UTC()
	state.Profile = profile
	state.LastStartedAt = &startedAt
	state.Status = market.SyncStatusRunning
	state.ErrorMessage = ""
	if err := synchronizer.store.SaveSyncState(ctx, state); err != nil {
		return fmt.Errorf("record running synchronization: %w", err)
	}

	items, err := synchronizer.exchange.ListInstruments(ctx)
	if err != nil {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("discover instruments: %w", err))
	}
	if len(items) == 0 {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("discover instruments: empty snapshot"))
	}
	if err := synchronizer.store.ApplyInstrumentSnapshot(ctx, items); err != nil {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("apply instrument snapshot: %w", err))
	}
	active, err := synchronizer.store.ListActiveInstruments(ctx)
	if err != nil {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("list active instruments: %w", err))
	}
	stats.instrumentsTotal = len(active)
	var instrumentFailures []error
	for _, instrument := range active {
		existing, err := synchronizer.store.ListLatestCandles(ctx, instrument.ID, 1)
		if err != nil {
			stats.instrumentsFailed++
			instrumentFailures = append(instrumentFailures, fmt.Errorf("inspect candle history for %s: %w", instrument.Symbol, err))
			continue
		}
		if len(existing) > 0 {
			stats.instrumentsSucceeded++
			continue
		}
		candles, err := synchronizer.exchange.ListClosedCandles(ctx, market.CandleRequest{
			Symbol: instrument.Symbol, Interval: profile.Interval, Limit: 30, ClosedBefore: startedAt,
		})
		if err != nil {
			stats.instrumentsFailed++
			instrumentFailures = append(instrumentFailures, fmt.Errorf("backfill candles for %s: %w", instrument.Symbol, err))
			continue
		}
		closed := make([]market.Candle, 0, len(candles))
		for _, candle := range candles {
			if !candle.CloseTime.Before(startedAt) {
				continue
			}
			candle.InstrumentID = instrument.ID
			candle.Interval = profile.Interval
			closed = append(closed, candle)
		}
		if err := synchronizer.store.UpsertCandles(ctx, closed); err != nil {
			stats.instrumentsFailed++
			instrumentFailures = append(instrumentFailures, fmt.Errorf("store candles for %s: %w", instrument.Symbol, err))
			continue
		}
		stats.instrumentsSucceeded++
		stats.candleRowsWritten += len(closed)
		for _, candle := range closed {
			if state.LastClosedOpenTime == nil || candle.OpenTime.After(*state.LastClosedOpenTime) {
				openTime := candle.OpenTime
				state.LastClosedOpenTime = &openTime
			}
		}
	}
	if len(instrumentFailures) > 0 {
		return synchronizer.recordFailure(ctx, &state, errors.Join(instrumentFailures...))
	}

	succeededAt := time.Now().UTC()
	previousSucceededAt := state.LastSucceededAt
	state.LastSucceededAt = &succeededAt
	state.Status = market.SyncStatusSucceeded
	if err := synchronizer.store.SaveSyncState(ctx, state); err != nil {
		state.LastSucceededAt = previousSucceededAt
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("record successful synchronization: %w", err))
	}
	return nil
}

type runStats struct {
	instrumentsTotal     int
	instrumentsSucceeded int
	instrumentsFailed    int
	candleRowsWritten    int
}

func (synchronizer *Synchronizer) recordFailure(ctx context.Context, state *market.SyncState, failure error) error {
	state.Status = market.SyncStatusFailed
	state.ErrorMessage = failure.Error()
	if err := synchronizer.store.SaveSyncState(ctx, *state); err != nil {
		return errors.Join(failure, fmt.Errorf("record failed synchronization: %w", err))
	}
	return failure
}
