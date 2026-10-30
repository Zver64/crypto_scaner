package strategy

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"crypto-scanner/internal/closedindicator"

	"github.com/google/uuid"
)

// roundWork is what one round evaluates each strategy against.
type roundWork struct {
	// instruments are the active ones; changed are those of them with
	// changed values, unless full evaluates every one.
	instruments, changed []Instrument
	full                 bool
	baselines            map[uuid.UUID]struct{}
	// missing are the targets some subscription has no value for yet.
	missing    []closedindicator.Target
	current    snapshot
	recipients []Recipient
	// catchUpLeft is how many more instruments may catch up this round.
	catchUpLeft *int
}

// check evaluates entry in one round: it halts a strategy whose coins are
// delisted, forgets the states of instruments it no longer evaluates, starts
// a requested baseline, or advances the candidates and alerts on the events
// it stores. Lagging candidates first catch up their missed candles, within
// the limit of the round and unless they wait for a catch-up retry; the
// others wait, neither caught up nor evaluated.
func (monitor *Monitor) check(ctx context.Context, state map[uuid.UUID]map[int64]TradeState, entry Entry, work roundWork) {
	// A delisted coin disables the strategy, which then waits for the
	// coin to be replaced.
	if absent := entry.inactive(work.instruments); len(absent) > 0 {
		monitor.halt(ctx, entry, notActive(absent))
		return
	}
	evaluated := entry.instruments(work.instruments)
	evaluates := make(map[int64]struct{}, len(evaluated))
	for _, instrument := range evaluated {
		evaluates[instrument.ID] = struct{}{}
	}
	states := state[entry.ID]
	if states == nil {
		states = map[int64]TradeState{}
		state[entry.ID] = states
	}
	// Instruments it no longer evaluates forget their states.
	var left []int64
	for id := range states {
		if _, ok := evaluates[id]; !ok {
			left = append(left, id)
		}
	}
	if len(left) > 0 {
		forgetFailure := failureKey{operation: "forget states", strategy: entry.ID}
		if err := monitor.store.DeleteStrategyStates(ctx, entry.ID, left); err != nil {
			monitor.failures.Warn(ctx, forgetFailure, "forget strategy states failed", "strategy_id", entry.ID, "error", err)
			monitor.storeFailed()
		} else {
			monitor.failures.Recover(ctx, forgetFailure, "forget strategy states recovered", "strategy_id", entry.ID)
			for _, id := range left {
				delete(states, id)
			}
		}
	}

	if _, baseline := work.baselines[entry.ID]; baseline {
		// The baseline waits for the first calculation of what it reads,
		// so it never misses an entry that is true; that calculation
		// reports changes, which evaluate again.
		if readsAny(entry, work.missing) {
			monitor.mu.Lock()
			monitor.baselines[entry.ID] = struct{}{}
			monitor.mu.Unlock()
			return
		}
		monitor.baseline(ctx, state, entry, evaluated, work.current, work.recipients)
		return
	}

	// A change of an instrument it reads through of changes its results
	// for every instrument.
	candidates := evaluated
	if !work.full && !slices.ContainsFunc(work.changed, func(instrument Instrument) bool { return slices.Contains(entry.symbols, instrument.Symbol) }) {
		candidates = slices.DeleteFunc(slices.Clone(work.changed), func(instrument Instrument) bool {
			_, ok := evaluates[instrument.ID]
			return !ok
		})
	}
	latest := entry.Interval.LastClosedOpenTime(work.current.now)
	retries := monitor.catchUps[entry.ID]
	// Instruments it no longer evaluates forget their catch-up retries.
	maps.DeleteFunc(retries, func(id int64, _ *catchUpRetry) bool {
		_, ok := evaluates[id]
		return !ok
	})
	plan := lagging(entry, candidates, states, latest, *work.catchUpLeft, func(id int64) bool {
		retry := retries[id]
		return retry != nil && work.current.now.Before(retry.at)
	})
	*work.catchUpLeft -= len(plan.behind)
	replays, loadErr := monitor.replays(ctx, entry, plan.behind, work.instruments, plan.from, latest)
	changes := map[int64]TradeState{}
	signals := map[int64][]alert{}
	var failure error
	for _, instrument := range candidates {
		if _, held := plan.held[instrument.ID]; held {
			continue
		}
		lags := slices.ContainsFunc(plan.behind, func(item Instrument) bool { return item.ID == instrument.ID })
		// A lagging instrument whose catch-up fails waits for its retry
		// instead of skipping its missed candles.
		if lags && loadErr != nil {
			monitor.catchUpFailed(ctx, entry, instrument, loadErr)
			continue
		}
		outOfRange := !entry.MarketCap.Contains(instrument.MarketCapUSD)
		result, err := monitor.catchUp(ctx, replays[instrument.ID], entry, instrument, states[instrument.ID], plan.from, latest, outOfRange)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			monitor.catchUpFailed(ctx, entry, instrument, err)
			continue
		}
		if result.halt == nil {
			result = result.advance(work.current, entry, instrument, outOfRange)
		}
		if result.halt != nil {
			failure = fmt.Errorf("%s %w", instrument.Symbol, result.halt)
			break
		}
		if lags {
			if result.processed {
				monitor.caughtUp(entry, instrument)
			} else {
				monitor.catchUpStalled(ctx, entry, instrument)
			}
		}
		if !result.processed {
			continue
		}
		changes[instrument.ID] = result.state
		// A coin new to the favorites starts silently from its entry now.
		if _, known := states[instrument.ID]; known || entry.CoinSource != CoinsFavorites {
			signals[instrument.ID] = result.alerts
		}
	}
	if failure != nil {
		monitor.halt(ctx, entry, failure)
		return
	}
	if len(plan.later) > 0 {
		// The instruments over the limit catch up in the next round.
		monitor.mu.Lock()
		for _, id := range plan.later {
			monitor.pending[id] = struct{}{}
		}
		monitor.mu.Unlock()
		monitor.signal()
	}
	if len(changes) == 0 {
		return
	}
	// Only states stored for a later candle alert, so a strategy changed
	// meanwhile, or a repeated evaluation, never alerts.
	stored, err := monitor.store.SaveStrategyStates(ctx, entry.ID, entry.Revision, changes)
	if err != nil {
		monitor.failures.Warn(ctx, storeFailure(entry), "store strategy states failed", "strategy_id", entry.ID, "error", err)
		monitor.storeFailed()
		return
	}
	monitor.failures.Recover(ctx, storeFailure(entry), "store strategy states recovered", "strategy_id", entry.ID)
	// States not stored, or a rejected stale revision, mean the local
	// state differs from the store.
	if len(stored) < len(changes) {
		monitor.desync()
	}
	var alerts []alert
	for _, instrument := range evaluated {
		if !slices.Contains(stored, instrument.ID) {
			continue
		}
		states[instrument.ID] = changes[instrument.ID]
		for _, item := range signals[instrument.ID] {
			if item.event.Kind != TradeSkip {
				alerts = append(alerts, item)
			}
		}
	}
	if len(alerts) > 0 {
		monitor.send(ctx, work.recipients, func(favorite func(string) bool) string { return alertText(entry, alerts, favorite) })
	}
}

// baseline starts the trading states of entry afresh from the entry values
// now, without trades, and announces the instruments within its market cap
// range whose entry is true. Instruments with unknown entries are left out and signal once their entry
// turns true.
func (monitor *Monitor) baseline(ctx context.Context, state map[uuid.UUID]map[int64]TradeState, entry Entry, instruments []Instrument, current snapshot, recipients []Recipient) {
	at := entry.Interval.LastClosedOpenTime(current.now)
	states := map[int64]TradeState{}
	var matching []Instrument
	for _, instrument := range instruments {
		result, known, err := current.match(entry.Compiled, entry, instrument.ID, nil)
		if err != nil {
			monitor.halt(ctx, entry, fmt.Errorf("%s %w", instrument.Symbol, ruleError(at, "the entry rule", err)))
			return
		}
		if !known {
			continue
		}
		states[instrument.ID] = TradeState{OpenTime: at, Entry: result}
		if result && entry.MarketCap.Contains(instrument.MarketCapUSD) {
			matching = append(matching, instrument)
		}
	}
	replaced, err := monitor.store.ReplaceStrategyStates(ctx, entry.ID, entry.Revision, states)
	if err != nil {
		// The transaction may have committed; the stored flag decides
		// whether the retry is still needed.
		monitor.failures.Warn(ctx, storeFailure(entry), "store strategy states failed", "strategy_id", entry.ID, "error", err)
		monitor.StrategiesChanged([]uuid.UUID{entry.ID})
		monitor.storeFailed()
		return
	}
	monitor.failures.Recover(ctx, storeFailure(entry), "store strategy states recovered", "strategy_id", entry.ID)
	if !replaced {
		// A newer change requested its own baseline, or an earlier attempt of
		// this one committed; either way the stored states are current.
		monitor.desync()
		return
	}
	state[entry.ID] = states
	monitor.send(ctx, recipients, func(favorite func(string) bool) string { return summaryText(entry, matching, current, favorite) })
}

// halt disables entry after err, a rule error or a coin that is no longer
// active, and tells the administrator.
// A disabled strategy is no longer evaluated, so the message goes once; a
// failed disable is retried by the next round, which meets the error again.
// A strategy changed since entry was listed stays enabled and is evaluated
// afresh. Its trading states go as a manual disable removes them.
func (monitor *Monitor) halt(ctx context.Context, entry Entry, err error) {
	disableFailure := failureKey{operation: "disable", strategy: entry.ID}
	disabled, disableErr := monitor.strategies.DisableAtRevision(ctx, entry.ID, entry.Revision)
	if disableErr != nil {
		monitor.failures.Warn(ctx, disableFailure, "disable halted strategy failed", "strategy_id", entry.ID, "reason", err, "error", disableErr)
		return
	}
	monitor.failures.Forget(disableFailure)
	if !disabled {
		return
	}
	monitor.logger.InfoContext(ctx, "strategy halted", "strategy_id", entry.ID, "error", err)
	monitor.send(ctx, []Recipient{{TelegramID: monitor.administratorID}}, func(func(string) bool) string { return haltText(entry, err) })
}

// ruleError names the rule and the candle, opening at at, of the rule error
// err, such as "at the candle opening 2026-03-02 10:00 UTC, the entry rule:
// the prev shift is 0, …".
func ruleError(at time.Time, rule string, err error) error {
	return fmt.Errorf("at the candle opening %s, %s: %w", candleTime(at), rule, err)
}

// storeFailure is the failure of storing the states of entry.
func storeFailure(entry Entry) failureKey {
	return failureKey{operation: "store states", strategy: entry.ID}
}
