package marketsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/market"
)

// Exchange is the discovery boundary required by instrument synchronization.
type Exchange interface {
	ListInstruments(context.Context) ([]market.Instrument, error)
	ListClosedCandles(context.Context, market.CandleRequest) ([]market.Candle, error)
	// RetryCount returns the cumulative number of retried exchange requests.
	RetryCount() uint64
}

// Store is the persistence boundary required by instrument synchronization.
type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	SaveSyncState(context.Context, market.SyncState) error
	ApplyInstrumentSnapshot(context.Context, []market.Instrument) error
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
	// SummarizeCandleHistory summarizes up to limit latest candles per
	// instrument; instruments without candles are omitted.
	SummarizeCandleHistory(context.Context, []int64, market.CandleInterval, int) (map[int64]HistorySummary, error)
	// Returns only committed insertions/corrections; unchanged rows are omitted.
	UpsertCandlesWithChanges(context.Context, []market.Candle) ([]market.Candle, error)
	GetCandleHistoryCoverage(context.Context, int64, market.CandleInterval) (market.HistoryCoverage, bool, error)
	SaveCandleHistoryCoverage(context.Context, market.HistoryCoverage) error
}

// HistorySummary describes the latest stored candles of one instrument, at
// most the synchronization depth of them.
type HistorySummary struct {
	Count          int
	Oldest, Latest time.Time
	// Gaps are internal holes only. Absence before Oldest may be the
	// instrument's pre-listing period and is not a gap.
	Gaps []HistoryGap
}

// HistoryGap is a hole between stored candles: From is the first missing open
// time and To the open time of the next stored candle.
type HistoryGap struct{ From, To time.Time }

// Synchronizer coordinates instrument discovery, backfill, and incremental loading.
type Synchronizer struct {
	exchange Exchange
	store    Store
	logger   *slog.Logger
	workers  int
	depth    int
	profile  market.SyncProfile
	runLock  sync.Mutex
}

const (
	exchangePageLimit           = 1000
	historyDepthPolicyVersion   = 1
	historyExhaustionRetryDelay = 7 * 24 * time.Hour
)

type intervalPolicy struct {
	interval     market.CandleInterval
	initialLimit int
	repairGaps   bool
}

func policyForInterval(interval market.CandleInterval, depth int) intervalPolicy {
	return intervalPolicy{interval: interval, initialLimit: depth, repairGaps: interval.Valid()}
}

// ErrSyncInProgress reports that another process-local synchronization owns the run lock.
var ErrSyncInProgress = errors.New("market synchronization already in progress")

// New creates a synchronizer for one independently persisted interval that
// backfills depth closed candles per instrument. The logger must be non-nil.
func New(exchange Exchange, store Store, logger *slog.Logger, workers, depth int, profile market.SyncProfile) *Synchronizer {
	if workers < 1 {
		workers = 1
	}
	return &Synchronizer{exchange: exchange, store: store, logger: logger, workers: workers, depth: max(depth, 1), profile: profile}
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
	retriesBefore := synchronizer.exchange.RetryCount()
	defer func() {
		stats.retryCount = synchronizer.exchange.RetryCount() - retriesBefore
		outcome := "succeeded"
		if syncErr != nil {
			outcome = "failed"
		}
		synchronizer.logger.InfoContext(ctx, "market synchronization completed",
			"module", "market_sync", "operation", "sync", "profile", profile.Key(),
			"duration", time.Since(operationStartedAt), "outcome", outcome,
			"instruments_total", stats.instrumentsTotal, "instruments_succeeded", stats.instrumentsSucceeded,
			"instruments_failed", stats.instrumentsFailed,
			"exchange_requests", stats.exchangeRequests, "candle_rows_requested", stats.candleRowsRequested,
			"candle_rows_written", stats.candleRowsWritten, "gap_ranges_repaired", stats.gapRangesRepaired,
			"lag_intervals", stats.lagIntervals, "retry_count", stats.retryCount,
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
	ids := make([]int64, len(active))
	for index, instrument := range active {
		ids[index] = instrument.ID
	}
	histories, err := synchronizer.store.SummarizeCandleHistory(ctx, ids, profile.Interval, synchronizer.depth)
	if err != nil {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("inspect candle history: %w", err))
	}
	results := synchronizer.syncInstruments(ctx, active, histories, profile, startedAt)
	var instrumentFailures []error
	for result := range results {
		if result.err != nil {
			stats.instrumentsFailed++
			instrumentFailures = append(instrumentFailures, result.err)
			continue
		}
		stats.instrumentsSucceeded++
		stats.exchangeRequests += result.exchangeRequests
		stats.candleRowsRequested += result.rowsRequested
		stats.candleRowsWritten += result.rowsWritten
		stats.gapRangesRepaired += result.gapRangesRepaired
		if result.latestOpenTime != nil && (state.LastClosedOpenTime == nil || result.latestOpenTime.After(*state.LastClosedOpenTime)) {
			state.LastClosedOpenTime = result.latestOpenTime
		}
	}
	if state.LastClosedOpenTime != nil {
		for open := state.LastClosedOpenTime.UTC(); open.Before(profile.Interval.LastClosedOpenTime(startedAt)); open = profile.Interval.NextOpenTime(open) {
			stats.lagIntervals++
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
	err               error
	exchangeRequests  int
	rowsRequested     int
	rowsWritten       int
	gapRangesRepaired int
	latestOpenTime    *time.Time
	oldestOpenTime    *time.Time
}

func (synchronizer *Synchronizer) syncInstruments(ctx context.Context, instruments []market.Instrument, histories map[int64]HistorySummary, profile market.SyncProfile, startedAt time.Time) <-chan instrumentResult {
	jobs := make(chan market.Instrument)
	results := make(chan instrumentResult)
	var workers sync.WaitGroup
	workerCount := min(synchronizer.workers, len(instruments))
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for instrument := range jobs {
				results <- synchronizer.syncInstrument(ctx, instrument, histories[instrument.ID], profile, startedAt)
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

func (synchronizer *Synchronizer) syncInstrument(ctx context.Context, instrument market.Instrument, history HistorySummary, profile market.SyncProfile, startedAt time.Time) instrumentResult {
	policy := policyForInterval(profile.Interval, synchronizer.depth)
	result := instrumentResult{}
	initiallyEmpty := history.Count == 0
	if initiallyEmpty {
		coverage, found, err := synchronizer.store.GetCandleHistoryCoverage(ctx, instrument.ID, profile.Interval)
		if err != nil {
			result.err = fmt.Errorf("load candle history coverage for %s: %w", instrument.Symbol, err)
			return result
		}
		if found && coverageCurrent(coverage, policy, startedAt) {
			return result
		}
		// The first page is the newest one; older pages follow as history repair.
		result = synchronizer.loadRange(ctx, instrument, market.CandleRequest{
			Symbol: instrument.Symbol, Interval: profile.Interval,
			Limit: min(policy.initialLimit, exchangePageLimit), ClosedBefore: startedAt,
		}, false)
		if result.err != nil {
			return result
		}
		if history, result.err = synchronizer.history(ctx, instrument, profile.Interval); result.err != nil {
			return result
		}
		if history.Count == 0 {
			result.err = synchronizer.saveExhaustedHistory(ctx, instrument, profile.Interval, policy, profile.Interval.LastClosedOpenTime(startedAt), startedAt)
			return result
		}
	}

	latest := history.Latest.UTC()
	result.latestOpenTime = laterOf(result.latestOpenTime, &latest)

	// Recheck the latest persisted close even when no newer interval exists.
	// Binance can correct an already stored final; overlapping the forward
	// cursor makes that correction durable and notifies active graphs.
	if !initiallyEmpty {
		previous := profile.Interval.PreviousOpenTime(latest)
		loaded := synchronizer.loadRange(ctx, instrument, market.CandleRequest{
			Symbol: instrument.Symbol, Interval: profile.Interval, Limit: exchangePageLimit,
			ClosedBefore: startedAt, AfterOpenTime: &previous,
		}, true)
		result = mergeInstrumentResults(result, loaded)
		if result.err != nil {
			return result
		}
	}
	if policy.repairGaps {
		for _, gap := range history.Gaps {
			after := gap.From.Add(-time.Millisecond)
			loaded := synchronizer.loadRange(ctx, instrument, market.CandleRequest{
				Symbol: instrument.Symbol, Interval: profile.Interval, Limit: exchangePageLimit,
				ClosedBefore: gap.To, AfterOpenTime: &after,
			}, true)
			loaded.gapRangesRepaired = 1
			result = mergeInstrumentResults(result, loaded)
			if result.err != nil {
				return result
			}
		}
	}
	// Forward and gap writes only add candles, so a reached depth stays reached.
	if history.Count >= policy.initialLimit {
		return result
	}

	// Forward and gap writes can change both the count and boundaries. The
	// persisted rows are the durable repair cursor, so decide depth from a
	// fresh read rather than from responses held in memory.
	if history, result.err = synchronizer.history(ctx, instrument, profile.Interval); result.err != nil {
		return result
	}
	if history.Count >= policy.initialLimit || history.Count == 0 {
		return result
	}
	oldest := history.Oldest.UTC()
	coverage, found, err := synchronizer.store.GetCandleHistoryCoverage(ctx, instrument.ID, profile.Interval)
	if err != nil {
		result.err = fmt.Errorf("load candle history coverage for %s: %w", instrument.Symbol, err)
		return result
	}
	if found && coverageCurrent(coverage, policy, startedAt) && coverage.VerifiedOldestOpenTime.Equal(oldest) {
		return result
	}

	// Load older pages until the depth is reached or the exchange runs out.
	for remaining := policy.initialLimit - history.Count; remaining > 0; {
		limit := min(remaining, exchangePageLimit)
		loaded := synchronizer.loadRange(ctx, instrument, market.CandleRequest{
			Symbol: instrument.Symbol, Interval: profile.Interval, Limit: limit,
			ClosedBefore: oldest, HistoryRepair: true,
		}, false)
		result = mergeInstrumentResults(result, loaded)
		if result.err != nil {
			return result
		}
		if loaded.rowsRequested < limit || loaded.oldestOpenTime == nil {
			verifiedOldest := *earlierOf(&oldest, loaded.oldestOpenTime)
			result.err = synchronizer.saveExhaustedHistory(ctx, instrument, profile.Interval, policy, verifiedOldest, startedAt)
			return result
		}
		remaining -= loaded.rowsRequested
		oldest = *loaded.oldestOpenTime
	}
	return result
}

// history reads the current summary of one instrument.
func (synchronizer *Synchronizer) history(ctx context.Context, instrument market.Instrument, interval market.CandleInterval) (HistorySummary, error) {
	histories, err := synchronizer.store.SummarizeCandleHistory(ctx, []int64{instrument.ID}, interval, synchronizer.depth)
	if err != nil {
		return HistorySummary{}, fmt.Errorf("inspect candle history for %s: %w", instrument.Symbol, err)
	}
	return histories[instrument.ID], nil
}

func (synchronizer *Synchronizer) loadRange(ctx context.Context, instrument market.Instrument, request market.CandleRequest, paginate bool) instrumentResult {
	result := instrumentResult{}
	for {
		result.exchangeRequests++
		candles, err := synchronizer.exchange.ListClosedCandles(ctx, request)
		if err != nil {
			result.err = fmt.Errorf("load candles for %s: %w", instrument.Symbol, err)
			return result
		}
		result.rowsRequested += len(candles)
		closed := make([]market.Candle, 0, len(candles))
		var pageLatest, pageOldest *time.Time
		for _, candle := range candles {
			if !candle.CloseTime.Before(request.ClosedBefore) || request.AfterOpenTime != nil && !candle.OpenTime.After(*request.AfterOpenTime) {
				continue
			}
			candle.InstrumentID = instrument.ID
			candle.Interval = request.Interval
			closed = append(closed, candle)
			openTime := candle.OpenTime.UTC()
			pageLatest = laterOf(pageLatest, &openTime)
			pageOldest = earlierOf(pageOldest, &openTime)
		}
		if _, err := synchronizer.store.UpsertCandlesWithChanges(ctx, closed); err != nil {
			result.err = fmt.Errorf("store candles for %s: %w", instrument.Symbol, err)
			return result
		}
		result.rowsWritten += len(closed)
		result.latestOpenTime = laterOf(result.latestOpenTime, pageLatest)
		result.oldestOpenTime = earlierOf(result.oldestOpenTime, pageOldest)
		if !paginate || len(candles) < request.Limit || pageLatest == nil {
			return result
		}
		request.AfterOpenTime = pageLatest
	}
}

func mergeInstrumentResults(current, addition instrumentResult) instrumentResult {
	current.exchangeRequests += addition.exchangeRequests
	current.rowsRequested += addition.rowsRequested
	current.rowsWritten += addition.rowsWritten
	current.gapRangesRepaired += addition.gapRangesRepaired
	if addition.err != nil {
		current.err = addition.err
	}
	current.latestOpenTime = laterOf(current.latestOpenTime, addition.latestOpenTime)
	current.oldestOpenTime = earlierOf(current.oldestOpenTime, addition.oldestOpenTime)
	return current
}

// laterOf returns the later of two optional times; nil means unknown.
func laterOf(current, candidate *time.Time) *time.Time {
	if candidate != nil && (current == nil || candidate.After(*current)) {
		return candidate
	}
	return current
}

// earlierOf returns the earlier of two optional times; nil means unknown.
func earlierOf(current, candidate *time.Time) *time.Time {
	if candidate != nil && (current == nil || candidate.Before(*current)) {
		return candidate
	}
	return current
}

// coverageCurrent reports whether persisted coverage still describes the depth policy
// and its exhaustion retry has not yet expired.
func coverageCurrent(coverage market.HistoryCoverage, policy intervalPolicy, at time.Time) bool {
	return coverage.TargetDepth == policy.initialLimit && coverage.PolicyVersion == historyDepthPolicyVersion && at.Before(coverage.RetryAfter)
}

// saveExhaustedHistory records that the exchange has no history older than
// verifiedOldest, so the backfill is not retried before the retry delay.
func (synchronizer *Synchronizer) saveExhaustedHistory(ctx context.Context, instrument market.Instrument, interval market.CandleInterval, policy intervalPolicy, verifiedOldest, at time.Time) error {
	err := synchronizer.store.SaveCandleHistoryCoverage(ctx, market.HistoryCoverage{
		InstrumentID: instrument.ID, Interval: interval, VerifiedOldestOpenTime: verifiedOldest,
		TargetDepth: policy.initialLimit, PolicyVersion: historyDepthPolicyVersion,
		RetryAfter: at.Add(historyExhaustionRetryDelay),
	})
	if err != nil {
		return fmt.Errorf("save candle history coverage for %s: %w", instrument.Symbol, err)
	}
	return nil
}

type runStats struct {
	instrumentsTotal     int
	instrumentsSucceeded int
	instrumentsFailed    int
	exchangeRequests     int
	candleRowsRequested  int
	candleRowsWritten    int
	gapRangesRepaired    int
	lagIntervals         int
	retryCount           uint64
}

func (synchronizer *Synchronizer) recordFailure(ctx context.Context, state *market.SyncState, failure error) error {
	state.Status = market.SyncStatusFailed
	state.ErrorMessage = failure.Error()
	if err := synchronizer.store.SaveSyncState(ctx, *state); err != nil {
		return errors.Join(failure, fmt.Errorf("record failed synchronization: %w", err))
	}
	return failure
}
