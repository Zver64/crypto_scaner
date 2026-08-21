package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
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
	ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error)
	UpsertCandles(context.Context, []market.Candle) error
}

// MVPProfile returns the code-owned Binance Spot synchronization profile.
func MVPProfile() market.SyncProfile {
	return market.SyncProfile{
		Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1d", TimeZone: "UTC",
	}
}

// HourlyProfile returns the independently synchronized hourly dataset.
func HourlyProfile() market.SyncProfile {
	return market.SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1h", TimeZone: "UTC"}
}

// Synchronizer coordinates instrument discovery, backfill, and incremental loading.
type Synchronizer struct {
	exchange Exchange
	store    Store
	logger   *slog.Logger
	workers  int
	profile  market.SyncProfile
	runLock  sync.Mutex
}

const defaultWorkerCount = 4

// ErrSyncInProgress reports that another process-local synchronization owns the run lock.
var ErrSyncInProgress = errors.New("market synchronization already in progress")

// New creates a market synchronizer that discards structured run logs.
func New(exchange Exchange, store Store) *Synchronizer {
	return NewWithLogger(exchange, store, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

// NewWithLogger creates a market synchronizer that emits per-run totals.
func NewWithLogger(exchange Exchange, store Store, logger *slog.Logger) *Synchronizer {
	return NewWithOptions(exchange, store, logger, defaultWorkerCount)
}

// NewWithOptions creates a synchronizer with explicit per-instance instrument concurrency.
func NewWithOptions(exchange Exchange, store Store, logger *slog.Logger, workers int) *Synchronizer {
	return NewWithProfile(exchange, store, logger, workers, MVPProfile())
}

// NewWithProfile creates a synchronizer for one independently persisted interval.
func NewWithProfile(exchange Exchange, store Store, logger *slog.Logger, workers int, profile market.SyncProfile) *Synchronizer {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	if workers < 1 {
		workers = 1
	}
	return &Synchronizer{exchange: exchange, store: store, logger: logger, workers: workers, profile: profile}
}

// Sync applies the catalog, backfills new instruments, and incrementally loads
// missing closed candles for instruments with history.
func (synchronizer *Synchronizer) Sync(ctx context.Context) (syncErr error) {
	if !synchronizer.runLock.TryLock() {
		return ErrSyncInProgress
	}
	defer synchronizer.runLock.Unlock()

	profile := synchronizer.profile
	operationStartedAt := time.Now()
	stats := runStats{}
	retriesBefore := synchronizer.retryCount()
	defer func() {
		stats.retryCount = synchronizer.retryCount() - retriesBefore
		outcome := "succeeded"
		if syncErr != nil {
			outcome = "failed"
		}
		synchronizer.logger.InfoContext(ctx, "market synchronization completed",
			"module", "market_sync", "operation", "sync", "profile", profile.Key(),
			"duration", time.Since(operationStartedAt), "outcome", outcome,
			"instruments_total", stats.instrumentsTotal, "instruments_succeeded", stats.instrumentsSucceeded,
			"instruments_failed", stats.instrumentsFailed, "candle_rows_written", stats.candleRowsWritten,
			"retry_count", stats.retryCount,
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
	results := synchronizer.syncInstruments(ctx, active, profile, startedAt)
	var instrumentFailures []error
	for result := range results {
		if result.err != nil {
			stats.instrumentsFailed++
			instrumentFailures = append(instrumentFailures, result.err)
			continue
		}
		stats.instrumentsSucceeded++
		stats.candleRowsWritten += result.rowsWritten
		if result.latestOpenTime != nil && (state.LastClosedOpenTime == nil || result.latestOpenTime.After(*state.LastClosedOpenTime)) {
			state.LastClosedOpenTime = result.latestOpenTime
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

type instrumentResult struct {
	err            error
	rowsWritten    int
	latestOpenTime *time.Time
}

func (synchronizer *Synchronizer) syncInstruments(ctx context.Context, instruments []market.Instrument, profile market.SyncProfile, startedAt time.Time) <-chan instrumentResult {
	jobs := make(chan market.Instrument)
	results := make(chan instrumentResult)
	var workers sync.WaitGroup
	workerCount := min(synchronizer.workers, len(instruments))
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for instrument := range jobs {
				results <- synchronizer.syncInstrument(ctx, instrument, profile, startedAt)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, instrument := range instruments {
			select {
			case jobs <- instrument:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()
	return results
}

func (synchronizer *Synchronizer) syncInstrument(ctx context.Context, instrument market.Instrument, profile market.SyncProfile, startedAt time.Time) instrumentResult {
	existing, err := synchronizer.store.ListLatestCandlesByInterval(ctx, instrument.ID, profile.Interval, 1)
	if err != nil {
		return instrumentResult{err: fmt.Errorf("inspect candle history for %s: %w", instrument.Symbol, err)}
	}
	initialLimit := 30
	if profile.Interval == "1h" {
		initialLimit = 60
	}
	request := market.CandleRequest{Symbol: instrument.Symbol, Interval: profile.Interval, Limit: initialLimit, ClosedBefore: startedAt}
	if len(existing) > 0 {
		latest := existing[0].OpenTime
		request.AfterOpenTime = &latest
		request.Limit = 1000
	}
	result := instrumentResult{}
	for {
		candles, err := synchronizer.exchange.ListClosedCandles(ctx, request)
		if err != nil {
			return instrumentResult{err: fmt.Errorf("load candles for %s: %w", instrument.Symbol, err)}
		}
		closed := make([]market.Candle, 0, len(candles))
		for _, candle := range candles {
			if !candle.CloseTime.Before(startedAt) || request.AfterOpenTime != nil && !candle.OpenTime.After(*request.AfterOpenTime) {
				continue
			}
			candle.InstrumentID = instrument.ID
			candle.Interval = profile.Interval
			closed = append(closed, candle)
		}
		if err := synchronizer.store.UpsertCandles(ctx, closed); err != nil {
			return instrumentResult{err: fmt.Errorf("store candles for %s: %w", instrument.Symbol, err)}
		}
		result.rowsWritten += len(closed)
		for _, candle := range closed {
			if result.latestOpenTime == nil || candle.OpenTime.After(*result.latestOpenTime) {
				openTime := candle.OpenTime
				result.latestOpenTime = &openTime
			}
		}
		if request.AfterOpenTime == nil || len(candles) < request.Limit || result.latestOpenTime == nil {
			return result
		}
		request.AfterOpenTime = result.latestOpenTime
	}
}

type runStats struct {
	instrumentsTotal     int
	instrumentsSucceeded int
	instrumentsFailed    int
	candleRowsWritten    int
	retryCount           uint64
}

type retryCounter interface {
	RetryCount() uint64
}

func (synchronizer *Synchronizer) retryCount() uint64 {
	if counter, ok := synchronizer.exchange.(retryCounter); ok {
		return counter.RetryCount()
	}
	return 0
}

func (synchronizer *Synchronizer) recordFailure(ctx context.Context, state *market.SyncState, failure error) error {
	state.Status = market.SyncStatusFailed
	state.ErrorMessage = failure.Error()
	if err := synchronizer.store.SaveSyncState(ctx, *state); err != nil {
		return errors.Join(failure, fmt.Errorf("record failed synchronization: %w", err))
	}
	return failure
}
