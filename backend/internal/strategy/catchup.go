package strategy

import (
	"context"
	"maps"
	"slices"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/backoff"

	"github.com/google/uuid"
)

// catchUpRetry delays the next catch-up of an instrument of a strategy
// whose last one failed or stalled.
type catchUpRetry struct {
	// failures counts the failed or stalled catch-ups in a row.
	failures int
	// at is when the next catch-up may start.
	at time.Time
}

// catchUpPlan is the catch-up of one strategy in one round.
type catchUpPlan struct {
	// behind are the lagging instruments that catch up now; held are those
	// that wait, either for their retry or, listed in later too, for the
	// next round.
	behind []Instrument
	held   map[int64]struct{}
	later  []int64
	// from is the open time of the oldest missed candle of behind, no older
	// than market.SyncDepth candles through latest allow.
	from time.Time
}

// lagging plans the catch-up of the candidates whose stored state processed
// a candle older than the one before latest, the latest closed candle of the
// interval of entry, so closed candles were missed, such as while the
// backend was down or when they were synchronized late. A state without a
// processed candle, or none, starts from the latest candle instead. Those
// waiting for their retry are held; of the others at most limit catch up
// now, those that missed the fewest candles first.
func lagging(entry Entry, candidates []Instrument, states map[int64]TradeState, latest time.Time, limit int, waiting func(int64) bool) catchUpPlan {
	plan := catchUpPlan{held: map[int64]struct{}{}, from: latest}
	var lags []Instrument
	for _, instrument := range candidates {
		if state, known := states[instrument.ID]; known && !state.OpenTime.IsZero() && state.OpenTime.Before(entry.Interval.PreviousOpenTime(latest)) {
			if waiting(instrument.ID) {
				plan.held[instrument.ID] = struct{}{}
			} else {
				lags = append(lags, instrument)
			}
		}
	}
	slices.SortStableFunc(lags, func(a, b Instrument) int { return states[b.ID].OpenTime.Compare(states[a.ID].OpenTime) })
	plan.behind = lags[:min(len(lags), max(limit, 0))]
	for _, instrument := range lags[len(plan.behind):] {
		plan.held[instrument.ID] = struct{}{}
		plan.later = append(plan.later, instrument.ID)
	}
	for _, instrument := range plan.behind {
		if missed := entry.Interval.NextOpenTime(states[instrument.ID].OpenTime); missed.Before(plan.from) {
			plan.from = missed
		}
	}
	earliest := latest
	for range market.SyncDepth - 1 {
		earliest = entry.Interval.PreviousOpenTime(earliest)
	}
	if plan.from.Before(earliest) {
		plan.from = earliest
	}
	return plan
}

// replays prepares a replay of entry for each of instruments that catches up
// the candles of its interval opening from from through latest; active are
// the instruments read through of. Each history is loaded as deep as the
// values of those candles need: the deepest window read on its interval and
// the candles of that interval the span covers.
func (monitor *Monitor) replays(ctx context.Context, entry Entry, instruments, active []Instrument, from, latest time.Time) (map[int64]*replay, error) {
	if len(instruments) == 0 {
		return nil, nil
	}
	result := make(map[int64]*replay, len(instruments))
	depths := map[market.CandleInterval]int{}
	for _, instrument := range instruments {
		replay, err := newReplay(monitor.registry, entry, instrument, active)
		if err != nil {
			return nil, err
		}
		for index, subscription := range replay.current.reads.subscriptions {
			depths[subscription.Target.Interval] = max(depths[subscription.Target.Interval], replay.depths[index])
		}
		result[instrument.ID] = replay
	}
	first, last := entry.Interval.NextOpenTime(from), entry.Interval.NextOpenTime(latest)
	limit := func(read market.CandleInterval) int {
		return depths[read] + read.CandlesBetween(read.LastClosedOpenTime(first), read.LastClosedOpenTime(last))
	}
	if err := loadHistories(ctx, monitor.candles, limit, slices.Collect(maps.Values(result))...); err != nil {
		return nil, err
	}
	return result, nil
}

// progress is the evaluation of one instrument in one round: the state it
// leaves, the alerts of the candles it processed, and whether it processed
// any. halt is a rule error that halts the strategy.
type progress struct {
	state     TradeState
	alerts    []alert
	processed bool
	halt      error
}

// catchUp steps the stored closed candles of the interval of entry on
// instrument that open after state, from from on, and before latest, oldest
// first, with the values replay calculates for each, so the trades are those
// a backtest of the same history makes. A nil replay steps nothing. Candles
// whose window is not warm, since the stored history misses older candles,
// are skipped like a backtest skips them; the candles skipped that way or
// before from are logged once. Its alerts name their candle. It fails on a
// failed calculation or a cancelled ctx.
func (monitor *Monitor) catchUp(ctx context.Context, replay *replay, entry Entry, instrument Instrument, state TradeState, from, latest time.Time, outOfRange bool) (progress, error) {
	result := progress{state: state}
	if replay == nil {
		return result, nil
	}
	missed := entry.Interval.NextOpenTime(state.OpenTime)
	// Candles before from lie beyond the span a catch-up covers.
	beyond := missed.Before(from)
	if !beyond {
		from = missed
	}
	// cold counts the skipped candles without a warm window; resumed is the
	// first candle stepped.
	cold := 0
	var resumed time.Time
	history := replay.histories[entry.Interval][instrument.ID]
	start, _ := slices.BinarySearchFunc(history, from, func(candle market.Candle, at time.Time) int { return candle.OpenTime.Compare(at) })
	for _, candle := range history[start:] {
		if !candle.OpenTime.Before(latest) {
			break
		}
		if err := ctx.Err(); err != nil {
			return progress{}, err
		}
		current, warm, err := replay.at(entry.Interval.NextOpenTime(candle.OpenTime))
		if err != nil {
			return progress{}, err
		}
		if !warm {
			cold++
			continue
		}
		if resumed.IsZero() {
			resumed = candle.OpenTime
		}
		next, events, ok, err := current.advance(entry, instrument.ID, result.state, outOfRange)
		if err != nil {
			return progress{state: state, halt: err}, nil
		}
		if !ok {
			continue
		}
		result.state, result.processed = next, true
		for _, event := range events {
			result.alerts = append(result.alerts, alert{instrument: instrument, event: event, past: candle.OpenTime.UTC()})
		}
	}
	if beyond || cold > 0 {
		monitor.logger.InfoContext(ctx, "strategy catch-up skipped missed candles", "strategy_id", entry.ID, "symbol", instrument.Symbol,
			"missed_from", missed.UTC(), "resumed_at", resumed.UTC(), "cold", cold)
	}
	return result, nil
}

// advance steps the latest closed candle of current after result.
func (result progress) advance(current snapshot, entry Entry, instrument Instrument, outOfRange bool) progress {
	next, events, stepped, err := current.advance(entry, instrument.ID, result.state, outOfRange)
	if err != nil {
		result.halt = err
		return result
	}
	if stepped {
		result.state, result.processed = next, true
	}
	for _, event := range events {
		result.alerts = append(result.alerts, alert{instrument: instrument, event: event})
	}
	return result
}

// catchUpFailed delays the catch-up of instrument by entry after err, a
// failed one: the instrument waits, neither caught up nor evaluated, until
// its retry delay passes, so no missed candle is skipped, while other
// instruments and strategies go on. Only the first failure since the
// instrument last caught up is warned about.
func (monitor *Monitor) catchUpFailed(ctx context.Context, entry Entry, instrument Instrument, err error) {
	monitor.delayCatchUp(entry.ID, instrument.ID)
	monitor.failures.Warn(ctx, catchUpFailure(entry, instrument), "strategy catch-up failed", "strategy_id", entry.ID, "symbol", instrument.Symbol, "error", err)
}

// catchUpStalled delays the catch-up of instrument by entry, like a failed
// one, after one that processed no candle, such as before the missed
// candles are synchronized, so it is not repeated every round.
func (monitor *Monitor) catchUpStalled(ctx context.Context, entry Entry, instrument Instrument) {
	retry := monitor.delayCatchUp(entry.ID, instrument.ID)
	monitor.logger.InfoContext(ctx, "strategy catch-up processed no candle", "strategy_id", entry.ID, "symbol", instrument.Symbol, "retry_at", retry.at.UTC())
}

// caughtUp forgets the retry of the catch-up of instrument by entry, and its
// failure.
func (monitor *Monitor) caughtUp(entry Entry, instrument Instrument) {
	monitor.failures.Forget(catchUpFailure(entry, instrument))
	delete(monitor.catchUps[entry.ID], instrument.ID)
	if len(monitor.catchUps[entry.ID]) == 0 {
		delete(monitor.catchUps, entry.ID)
	}
}

// delayCatchUp delays the next catch-up of instrumentID by strategy by the
// backoff of its failures in a row.
func (monitor *Monitor) delayCatchUp(strategy uuid.UUID, instrumentID int64) *catchUpRetry {
	retries := monitor.catchUps[strategy]
	if retries == nil {
		retries = map[int64]*catchUpRetry{}
		monitor.catchUps[strategy] = retries
	}
	retry := retries[instrumentID]
	if retry == nil {
		retry = &catchUpRetry{}
		retries[instrumentID] = retry
	}
	retry.at = monitor.now().Add(backoff.Exponential(retryDelay, maxRetryDelay, retry.failures))
	retry.failures++
	return retry
}

// nextCatchUp returns the earliest retry of a delayed catch-up still to
// come.
func (monitor *Monitor) nextCatchUp() (time.Time, bool) {
	now := monitor.now()
	var next time.Time
	for _, retries := range monitor.catchUps {
		for _, retry := range retries {
			if retry.at.After(now) && (next.IsZero() || retry.at.Before(next)) {
				next = retry.at
			}
		}
	}
	return next, !next.IsZero()
}

// catchUpFailure is the failure of the catch-up of instrument by entry.
func catchUpFailure(entry Entry, instrument Instrument) failureKey {
	return failureKey{operation: "catch-up", strategy: entry.ID, instrument: instrument.ID}
}
