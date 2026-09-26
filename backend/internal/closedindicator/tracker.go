// Package closedindicator keeps indicator values at the latest closed candle
// current in the background and serves the same values to tables.
package closedindicator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
)

const (
	// minimumHistory matches the initial closed range of a chart, so a table
	// value equals the last closed point of that chart on the same history.
	minimumHistory = 200
	retryDelay     = 30 * time.Second
	refreshPeriod  = 5 * time.Minute
)

// Target is an indicator selection calculated on one candle interval.
type Target struct {
	Interval  market.CandleInterval
	Selection indicator.Selection
}

func (target Target) id() string {
	parameters, _ := json.Marshal(target.Selection.Parameters)
	return string(target.Interval) + "|" + string(target.Selection.Type) + "|" + string(parameters)
}

type Output struct {
	Name  string
	Value float64
}

// Value holds the named outputs at the latest stored closed candle. Outputs is
// empty when the continuous history ending at that candle is too short.
type Value struct {
	Target   Target
	OpenTime time.Time
	Outputs  []Output
}

func (value Value) equal(other Value) bool {
	return value.OpenTime.Equal(other.OpenTime) && slices.Equal(value.Outputs, other.Outputs)
}

// Change reports a background recalculation of a tracked pair whose value
// differs from the last known one. Previous is zero when nothing was known.
type Change struct {
	InstrumentID int64
	Previous     Value
	Current      Value
}

// Subscription requests background tracking of one target for one instrument.
type Subscription struct {
	InstrumentID int64
	Target       Target
}

// Source supplies the pairs that must stay current without any clients.
// targets are the tracker's table targets; a source may track other ones.
type Source interface {
	Subscriptions(ctx context.Context, targets []Target) ([]Subscription, error)
}

// InstrumentSource tracks every table target for each listed instrument.
type InstrumentSource func(context.Context) ([]int64, error)

func (source InstrumentSource) Subscriptions(ctx context.Context, targets []Target) ([]Subscription, error) {
	ids, err := source(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Subscription, 0, len(ids)*len(targets))
	for _, id := range ids {
		for _, target := range targets {
			result = append(result, Subscription{InstrumentID: id, Target: target})
		}
	}
	return result, nil
}

type Store interface {
	// ListLatestCandles returns up to limit latest closed candles per
	// instrument in chronological order.
	ListLatestCandles(context.Context, []int64, market.CandleInterval, int) (map[int64][]market.Candle, error)
}

type pairKey struct {
	instrumentID int64
	target       string
}

type historyKey struct {
	instrumentID int64
	interval     market.CandleInterval
}

// Tracker recalculates tracked pairs whenever their closed history changes and
// caches on-demand table values until the history of their instrument changes.
type Tracker struct {
	store    Store
	registry *indicator.Registry
	targets  []Target
	sources  []Source
	logger   *slog.Logger
	wake     chan struct{}
	// retries are applied by Run after retryDelay; only Run's goroutine
	// touches them.
	retries []func()

	mu        sync.Mutex
	tracked   map[pairKey]Subscription
	values    map[pairKey]Value
	versions  map[historyKey]uint64
	dirty     map[historyKey]struct{}
	refresh   bool
	listeners []func(Change)
}

// New creates a tracker. Targets are the values served to tables; sources
// decide which pairs are kept current in the background.
func New(store Store, registry *indicator.Registry, targets []Target, logger *slog.Logger, sources ...Source) (*Tracker, error) {
	if store == nil || registry == nil || logger == nil {
		return nil, fmt.Errorf("closed indicator store, indicator registry, and logger are required")
	}
	tracker := &Tracker{
		store: store, registry: registry, targets: targets, sources: sources, logger: logger,
		wake: make(chan struct{}, 1), tracked: map[pairKey]Subscription{}, values: map[pairKey]Value{},
		versions: map[historyKey]uint64{}, dirty: map[historyKey]struct{}{},
	}
	for _, target := range targets {
		if _, err := tracker.depth(target); err != nil {
			return nil, err
		}
	}
	return tracker, nil
}

// Listen registers a consumer of recalculated tracked values. It is called
// outside the tracker lock and must not block.
func (tracker *Tracker) Listen(listener func(Change)) {
	tracker.mu.Lock()
	tracker.listeners = append(tracker.listeners, listener)
	tracker.mu.Unlock()
}

// Refresh reloads the tracked pairs from all sources.
func (tracker *Tracker) Refresh() {
	tracker.mu.Lock()
	tracker.refresh = true
	tracker.mu.Unlock()
	tracker.signal()
}

// HistoryChanged invalidates values after committed closed-candle changes. It
// never waits for recalculation.
func (tracker *Tracker) HistoryChanged(candles []market.Candle) {
	touched := map[historyKey]struct{}{}
	for _, candle := range candles {
		touched[historyKey{candle.InstrumentID, candle.Interval}] = struct{}{}
	}
	tracker.mu.Lock()
	marked := false
	for key := range touched {
		tracker.versions[key]++
	}
	for pair, subscription := range tracker.tracked {
		key := historyKey{pair.instrumentID, subscription.Target.Interval}
		if _, ok := touched[key]; ok {
			tracker.dirty[key] = struct{}{}
			marked = true
		}
	}
	// Tracked values stay available until recalculated; cached ones expire.
	for pair, value := range tracker.values {
		if _, ok := touched[historyKey{pair.instrumentID, value.Target.Interval}]; ok {
			if _, tracked := tracker.tracked[pair]; !tracked {
				delete(tracker.values, pair)
			}
		}
	}
	tracker.mu.Unlock()
	if marked {
		tracker.signal()
	}
}

// Latest returns the table targets for each instrument, in target order. A
// value that cannot be loaded has no outputs, so a table shows it as missing.
func (tracker *Tracker) Latest(ctx context.Context, instrumentIDs []int64) map[int64][]Value {
	result := make(map[int64][]Value, len(instrumentIDs))
	var missing []Subscription
	tracker.mu.Lock()
	for _, id := range instrumentIDs {
		for _, target := range tracker.targets {
			if _, ok := tracker.values[pairKey{id, target.id()}]; !ok {
				missing = append(missing, Subscription{InstrumentID: id, Target: target})
			}
		}
	}
	versions := tracker.versionSnapshot(missing)
	tracker.mu.Unlock()
	computed := map[pairKey]Value{}
	failed := false
	if len(missing) > 0 {
		var err error
		if computed, err = tracker.calculate(ctx, missing); err != nil {
			tracker.logger.WarnContext(ctx, "load closed indicators failed", "module", "closed_indicator", "error", err)
			failed = true
		}
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for _, subscription := range missing {
		key := pairKey{subscription.InstrumentID, subscription.Target.id()}
		history := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		if _, exists := tracker.values[key]; !failed && !exists && tracker.versions[history] == versions[history] {
			tracker.values[key] = computed[key]
		}
	}
	for _, id := range instrumentIDs {
		values := make([]Value, len(tracker.targets))
		for index, target := range tracker.targets {
			key := pairKey{id, target.id()}
			if value, ok := computed[key]; ok {
				values[index] = value
			} else if value, ok := tracker.values[key]; ok {
				values[index] = value
			} else {
				values[index] = Value{Target: target}
			}
		}
		result[id] = values
	}
	return result
}

// Run keeps tracked pairs current until ctx is cancelled.
func (tracker *Tracker) Run(ctx context.Context) error {
	tracker.Refresh()
	ticker := time.NewTicker(refreshPeriod)
	defer ticker.Stop()
	retry := time.NewTimer(retryDelay)
	retry.Stop()
	defer retry.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			tracker.Refresh()
		case <-retry.C:
			tracker.mu.Lock()
			for _, mark := range tracker.retries {
				mark()
			}
			tracker.mu.Unlock()
			tracker.retries = nil
			tracker.step(ctx)
			if len(tracker.retries) > 0 {
				retry.Reset(retryDelay)
			}
		case <-tracker.wake:
			scheduled := len(tracker.retries) > 0
			tracker.step(ctx)
			if !scheduled && len(tracker.retries) > 0 {
				retry.Reset(retryDelay)
			}
		}
	}
}

func (tracker *Tracker) signal() {
	select {
	case tracker.wake <- struct{}{}:
	default:
	}
}

func (tracker *Tracker) step(ctx context.Context) {
	tracker.mu.Lock()
	refresh, dirty := tracker.refresh, tracker.dirty
	tracker.refresh, tracker.dirty = false, map[historyKey]struct{}{}
	tracker.mu.Unlock()
	pending := map[pairKey]Subscription{}
	if refresh {
		subscriptions, err := tracker.collect(ctx)
		if err != nil {
			tracker.logger.WarnContext(ctx, "load tracked closed indicators failed", "module", "closed_indicator", "error", err)
			tracker.retry(func() { tracker.refresh = true })
		} else {
			tracker.mu.Lock()
			next := make(map[pairKey]Subscription, len(subscriptions))
			for _, subscription := range subscriptions {
				key := pairKey{subscription.InstrumentID, subscription.Target.id()}
				next[key] = subscription
				if _, tracked := tracker.tracked[key]; !tracked {
					pending[key] = subscription
				}
			}
			// A dropped pair may hold a value awaiting recalculation, which
			// would otherwise be served as a fresh cache entry.
			for key := range tracker.tracked {
				if _, kept := next[key]; !kept {
					delete(tracker.values, key)
				}
			}
			tracker.tracked = next
			tracker.mu.Unlock()
		}
	}
	tracker.mu.Lock()
	for key, subscription := range tracker.tracked {
		if _, ok := dirty[historyKey{key.instrumentID, subscription.Target.Interval}]; ok {
			pending[key] = subscription
		}
	}
	subscriptions := make([]Subscription, 0, len(pending))
	for _, subscription := range pending {
		subscriptions = append(subscriptions, subscription)
	}
	versions := tracker.versionSnapshot(subscriptions)
	tracker.mu.Unlock()
	if len(subscriptions) == 0 {
		return
	}
	computed, err := tracker.calculate(ctx, subscriptions)
	if err != nil {
		tracker.logger.WarnContext(ctx, "recalculate closed indicators failed", "module", "closed_indicator", "error", err)
		tracker.retry(func() {
			for _, subscription := range subscriptions {
				tracker.dirty[historyKey{subscription.InstrumentID, subscription.Target.Interval}] = struct{}{}
			}
		})
		return
	}
	var changes []Change
	tracker.mu.Lock()
	for key, subscription := range pending {
		history := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		if _, tracked := tracker.tracked[key]; !tracked {
			delete(tracker.values, key)
			continue
		}
		// A newer history change has already queued another recalculation.
		if tracker.versions[history] != versions[history] {
			continue
		}
		previous, existed := tracker.values[key]
		current := computed[key]
		tracker.values[key] = current
		if !existed || !previous.equal(current) {
			changes = append(changes, Change{InstrumentID: subscription.InstrumentID, Previous: previous, Current: current})
		}
	}
	listeners := slices.Clone(tracker.listeners)
	tracker.mu.Unlock()
	for _, change := range changes {
		for _, listener := range listeners {
			listener(change)
		}
	}
}

func (tracker *Tracker) collect(ctx context.Context) ([]Subscription, error) {
	var result []Subscription
	for _, source := range tracker.sources {
		subscriptions, err := source.Subscriptions(ctx, tracker.targets)
		if err != nil {
			return nil, err
		}
		for _, subscription := range subscriptions {
			if _, err := tracker.depth(subscription.Target); err != nil {
				tracker.logger.WarnContext(ctx, "skip invalid closed indicator subscription", "module", "closed_indicator", "instrument_id", subscription.InstrumentID, "error", err)
				continue
			}
			result = append(result, subscription)
		}
	}
	return result, nil
}

// retry schedules mark to run under the lock after retryDelay, followed by a
// recalculation step.
func (tracker *Tracker) retry(mark func()) {
	tracker.retries = append(tracker.retries, mark)
}

// versionSnapshot must be called with the lock held.
func (tracker *Tracker) versionSnapshot(subscriptions []Subscription) map[historyKey]uint64 {
	result := make(map[historyKey]uint64, len(subscriptions))
	for _, subscription := range subscriptions {
		key := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		result[key] = tracker.versions[key]
	}
	return result
}

func (tracker *Tracker) depth(target Target) (int, error) {
	if !target.Interval.Valid() {
		return 0, fmt.Errorf("closed indicator interval %q is unsupported", target.Interval)
	}
	lookback, err := tracker.registry.Lookback(target.Selection.Type, target.Selection.Parameters)
	if err != nil {
		return 0, fmt.Errorf("closed indicator %q: %w", target.Selection.Type, err)
	}
	return max(minimumHistory, lookback+1), nil
}

// calculate batch-loads closed history per target and evaluates each pair.
func (tracker *Tracker) calculate(ctx context.Context, subscriptions []Subscription) (map[pairKey]Value, error) {
	groups := map[string][]Subscription{}
	for _, subscription := range subscriptions {
		id := subscription.Target.id()
		groups[id] = append(groups[id], subscription)
	}
	result := make(map[pairKey]Value, len(subscriptions))
	for id, group := range groups {
		target := group[0].Target
		depth, err := tracker.depth(target)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, len(group))
		for index, subscription := range group {
			ids[index] = subscription.InstrumentID
		}
		candles, err := tracker.store.ListLatestCandles(ctx, ids, target.Interval, depth)
		if err != nil {
			return nil, fmt.Errorf("load closed %s history: %w", target.Interval, err)
		}
		for _, instrumentID := range ids {
			value, err := tracker.value(target, candles[instrumentID])
			if err != nil {
				return nil, err
			}
			result[pairKey{instrumentID, id}] = value
		}
	}
	return result, nil
}

func (tracker *Tracker) value(target Target, candles []market.Candle) (Value, error) {
	value := Value{Target: target}
	if len(candles) == 0 {
		return value, nil
	}
	last := candles[len(candles)-1].OpenTime.UTC()
	value.OpenTime = last
	results, err := tracker.registry.CalculateCandles(target.Interval, candles, []indicator.Selection{target.Selection})
	if err != nil {
		return Value{}, fmt.Errorf("calculate closed %s: %w", target.Selection.Type, err)
	}
	for _, series := range results[0].Series {
		if count := len(series.Points); count > 0 && series.Points[count-1].Time.Equal(last) &&
			numeric.Finite(series.Points[count-1].Value) {
			value.Outputs = append(value.Outputs, Output{Name: series.Name, Value: series.Points[count-1].Value})
		}
	}
	return value, nil
}
