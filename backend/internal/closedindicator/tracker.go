// Package closedindicator keeps indicator values at the latest closed candle
// current in the background and serves the same values to tables.
package closedindicator

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
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

// Equal reports whether both targets identify the same interval and selection.
func (target Target) Equal(other Target) bool { return target.Key() == other.Key() }

// Key identifies the interval and the canonical selection of target.
func (target Target) Key() string {
	var key strings.Builder
	key.WriteString(string(target.Interval) + "|" + string(target.Selection.Type))
	for _, name := range slices.Sorted(maps.Keys(target.Selection.Parameters)) {
		fmt.Fprintf(&key, "|%s=%v", name, target.Selection.Parameters[name])
	}
	return key.String()
}

// Output is one named output at the latest stored closed candle. Earlier
// holds its values at the closed candles before it, newest first, as far as
// the requested points reach and the history is continuous.
type Output struct {
	Name    string
	Value   float64
	Earlier []float64
}

// At returns the value shift closed candles before the latest one.
func (output Output) At(shift int) (float64, bool) {
	if shift == 0 {
		return output.Value, true
	}
	if shift < 0 || shift > len(output.Earlier) {
		return 0, false
	}
	return output.Earlier[shift-1], true
}

// Value holds the named outputs at the latest stored closed candle. Outputs is
// empty when the continuous history ending at that candle is too short.
type Value struct {
	Target   Target
	OpenTime time.Time
	Outputs  []Output
	// points is the number of points the value was calculated for.
	points int
}

func (value Value) equal(other Value) bool {
	return value.OpenTime.Equal(other.OpenTime) && slices.EqualFunc(value.Outputs, other.Outputs, func(left, right Output) bool {
		return left.Name == right.Name && left.Value == right.Value && slices.Equal(left.Earlier, right.Earlier)
	})
}

// Change reports a background recalculation of a tracked pair whose value
// differs from the last known one. Previous is zero when nothing was known.
type Change struct {
	InstrumentID int64
	Previous     Value
	Current      Value
}

// Subscription requests background tracking of one target for one instrument.
// Points is the number of latest closed candles whose values are kept; zero
// keeps only the latest one.
type Subscription struct {
	InstrumentID int64
	Target       Target
	Points       int
}

func (subscription Subscription) points() int { return max(1, subscription.Points) }

// Source supplies the pairs that must stay current without any clients.
// targets are the tracker's table targets; a source may track other ones.
type Source interface {
	Subscriptions(ctx context.Context, targets []Target) ([]Subscription, error)
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

// Tracker recalculates tracked pairs once a synchronization round changed
// their closed history and caches on-demand table values until the history of
// their instrument changes.
type Tracker struct {
	store    Store
	registry *indicator.Registry
	sources  []Source
	logger   *slog.Logger
	wake     chan struct{}
	// retries are applied by Run after retryDelay; only Run's goroutine
	// touches them.
	retries []func()

	mu      sync.Mutex
	targets []Target
	// tableKeys holds the keys of targets by interval; every cached value
	// that is not tracked belongs to one of them.
	tableKeys map[market.CandleInterval][]string
	tracked   map[pairKey]Subscription
	// trackedHistories are the histories of the tracked pairs.
	trackedHistories map[historyKey]struct{}
	values           map[pairKey]Value
	versions         map[historyKey]uint64
	dirty            map[historyKey]struct{}
	// synced lets the next step recalculate the dirty histories, which
	// otherwise wait for the end of the synchronization round.
	synced    bool
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
		store: store, registry: registry, targets: targets, tableKeys: keysByInterval(targets), sources: sources, logger: logger,
		wake: make(chan struct{}, 1), tracked: map[pairKey]Subscription{}, trackedHistories: map[historyKey]struct{}{}, values: map[pairKey]Value{},
		versions: map[historyKey]uint64{}, dirty: map[historyKey]struct{}{},
	}
	for _, target := range targets {
		if _, err := Depth(registry, target, 1); err != nil {
			return nil, err
		}
	}
	return tracker, nil
}

func keysByInterval(targets []Target) map[market.CandleInterval][]string {
	result := map[market.CandleInterval][]string{}
	for _, target := range targets {
		result[target.Interval] = append(result[target.Interval], target.Key())
	}
	return result
}

// Listen registers a consumer of recalculated tracked values. It is called
// outside the tracker lock and must not block.
func (tracker *Tracker) Listen(listener func(Change)) {
	tracker.mu.Lock()
	tracker.listeners = append(tracker.listeners, listener)
	tracker.mu.Unlock()
}

// SetTargets replaces the table targets, drops cached values of removed ones,
// and reloads the tracked pairs.
func (tracker *Tracker) SetTargets(targets []Target) error {
	for _, target := range targets {
		if _, err := Depth(tracker.registry, target, 1); err != nil {
			return err
		}
	}
	kept := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		kept[target.Key()] = struct{}{}
	}
	tracker.mu.Lock()
	tracker.targets = slices.Clone(targets)
	tracker.tableKeys = keysByInterval(targets)
	for pair := range tracker.values {
		if _, ok := kept[pair.target]; !ok {
			if _, tracked := tracker.tracked[pair]; !tracked {
				delete(tracker.values, pair)
			}
		}
	}
	tracker.refresh = true
	tracker.mu.Unlock()
	tracker.signal()
	return nil
}

// Refresh reloads the tracked pairs from all sources.
func (tracker *Tracker) Refresh() {
	tracker.mu.Lock()
	tracker.refresh = true
	tracker.mu.Unlock()
	tracker.signal()
}

// HistoryChanged invalidates values after committed closed-candle changes and
// marks tracked histories for the recalculation HistorySynced starts. It never
// waits for recalculation.
func (tracker *Tracker) HistoryChanged(candles []market.Candle) {
	touched := map[historyKey]struct{}{}
	for _, candle := range candles {
		touched[historyKey{candle.InstrumentID, candle.Interval}] = struct{}{}
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for key := range touched {
		tracker.versions[key]++
		if _, ok := tracker.trackedHistories[key]; ok {
			tracker.dirty[key] = struct{}{}
		}
		// Tracked values stay available until recalculated; cached ones
		// expire.
		for _, target := range tracker.tableKeys[key.interval] {
			pair := pairKey{key.instrumentID, target}
			if _, tracked := tracker.tracked[pair]; !tracked {
				delete(tracker.values, pair)
			}
		}
	}
}

// HistorySynced recalculates the tracked pairs whose history changed, once a
// synchronization round has committed all its changes. It never blocks.
func (tracker *Tracker) HistorySynced() {
	tracker.mu.Lock()
	marked := len(tracker.dirty) > 0
	tracker.synced = tracker.synced || marked
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
	targets := tracker.targets
	keys := make([]string, len(targets))
	for index, target := range targets {
		keys[index] = target.Key()
	}
	for _, id := range instrumentIDs {
		for index, target := range targets {
			if _, ok := tracker.values[pairKey{id, keys[index]}]; !ok {
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
		key := pairKey{subscription.InstrumentID, subscription.Target.Key()}
		history := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		// A value calculated before a history change is stale; a newer
		// cached value or none is returned instead.
		if tracker.versions[history] != versions[history] {
			delete(computed, key)
			continue
		}
		// Only values of current targets are cached, so history changes
		// expire every cached value.
		current := slices.Contains(tracker.tableKeys[subscription.Target.Interval], key.target)
		if _, exists := tracker.values[key]; !failed && !exists && current {
			tracker.values[key] = computed[key]
		}
	}
	for _, id := range instrumentIDs {
		values := make([]Value, len(targets))
		for index, target := range targets {
			key := pairKey{id, keys[index]}
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

// Snapshot returns the known value of each subscription, in subscription
// order, without calculating anything. A pair that is neither tracked nor
// cached has no outputs, and its target is listed in missing, so results that
// depend on it are not final yet; so is a target whose value was calculated
// for fewer points than subscribed.
func (tracker *Tracker) Snapshot(subscriptions []Subscription) (values []Value, missing []Target) {
	values = make([]Value, len(subscriptions))
	absent := map[string]Target{}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for index, subscription := range subscriptions {
		key := subscription.Target.Key()
		value, ok := tracker.values[pairKey{subscription.InstrumentID, key}]
		if !ok {
			value = Value{Target: subscription.Target}
			absent[key] = subscription.Target
		} else if value.points < subscription.points() {
			absent[key] = subscription.Target
		}
		values[index] = value
	}
	for _, key := range slices.Sorted(maps.Keys(absent)) {
		missing = append(missing, absent[key])
	}
	return values, missing
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
	refresh := tracker.refresh
	tracker.refresh = false
	var dirty map[historyKey]struct{}
	if tracker.synced {
		dirty = tracker.dirty
		tracker.synced, tracker.dirty = false, map[historyKey]struct{}{}
	}
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
				key := pairKey{subscription.InstrumentID, subscription.Target.Key()}
				if existing, ok := next[key]; ok {
					subscription.Points = max(subscription.points(), existing.points())
				}
				next[key] = subscription
			}
			// A pair that now keeps more points is recalculated as well.
			for key, subscription := range next {
				if existing, tracked := tracker.tracked[key]; !tracked || existing.points() != subscription.points() {
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
			tracker.trackedHistories = make(map[historyKey]struct{}, len(next))
			for key, subscription := range next {
				tracker.trackedHistories[historyKey{key.instrumentID, subscription.Target.Interval}] = struct{}{}
			}
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
			tracker.synced = true
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
	tracker.mu.Lock()
	targets := tracker.targets
	tracker.mu.Unlock()
	var result []Subscription
	for _, source := range tracker.sources {
		subscriptions, err := source.Subscriptions(ctx, targets)
		if err != nil {
			return nil, err
		}
		for _, subscription := range subscriptions {
			if _, err := Depth(tracker.registry, subscription.Target, subscription.points()); err != nil {
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

// Depth is the number of closed candles loaded to calculate points values of
// target. Every point gets the warm-up of the latest one, so unstable
// indicators agree with charts on all of them. A depth beyond
// market.HistoryDepth only loads the kept candles, so the values it needs
// stay missing.
func Depth(registry *indicator.Registry, target Target, points int) (int, error) {
	if !target.Interval.Valid() {
		return 0, fmt.Errorf("closed indicator interval %q is unsupported", target.Interval)
	}
	lookback, err := registry.Lookback(target.Selection.Type, target.Selection.Parameters)
	if err != nil {
		return 0, fmt.Errorf("closed indicator %q: %w", target.Selection.Type, err)
	}
	return max(minimumHistory, lookback+1) + max(1, points) - 1, nil
}

// calculate loads the closed history of each interval once, as deep as its
// deepest pair needs, and evaluates every pair over its own latest candles.
func (tracker *Tracker) calculate(ctx context.Context, subscriptions []Subscription) (map[pairKey]Value, error) {
	type group struct {
		ids   []int64
		seen  map[int64]struct{}
		depth int
	}
	groups := map[market.CandleInterval]*group{}
	depths := make([]int, len(subscriptions))
	for index, subscription := range subscriptions {
		depth, err := Depth(tracker.registry, subscription.Target, subscription.points())
		if err != nil {
			return nil, err
		}
		depths[index] = depth
		interval := subscription.Target.Interval
		if groups[interval] == nil {
			groups[interval] = &group{seen: map[int64]struct{}{}}
		}
		current := groups[interval]
		current.depth = max(current.depth, depth)
		if _, ok := current.seen[subscription.InstrumentID]; !ok {
			current.seen[subscription.InstrumentID] = struct{}{}
			current.ids = append(current.ids, subscription.InstrumentID)
		}
	}
	histories := make(map[market.CandleInterval]map[int64][]market.Candle, len(groups))
	for interval, current := range groups {
		candles, err := tracker.store.ListLatestCandles(ctx, current.ids, interval, current.depth)
		if err != nil {
			return nil, fmt.Errorf("load closed %s history: %w", interval, err)
		}
		histories[interval] = candles
	}
	result := make(map[pairKey]Value, len(subscriptions))
	for index, subscription := range subscriptions {
		candles := histories[subscription.Target.Interval][subscription.InstrumentID]
		candles = candles[max(0, len(candles)-depths[index]):]
		value, err := tracker.value(subscription.Target, candles, subscription.points())
		if err != nil {
			return nil, err
		}
		result[pairKey{subscription.InstrumentID, subscription.Target.Key()}] = value
	}
	return result, nil
}

// value reads the outputs at the latest candle and up to points−1 earlier
// ones.
func (tracker *Tracker) value(target Target, candles []market.Candle, points int) (Value, error) {
	value := Value{Target: target, points: points}
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
		count := len(series.Points)
		if count == 0 || !series.Points[count-1].Time.Equal(last) || !numeric.Finite(series.Points[count-1].Value) {
			continue
		}
		output := Output{Name: series.Name, Value: series.Points[count-1].Value}
		expected := last
		for index := count - 2; index >= 0 && len(output.Earlier) < points-1; index-- {
			expected = target.Interval.PreviousOpenTime(expected)
			point := series.Points[index]
			if !point.Time.Equal(expected) || !numeric.Finite(point.Value) {
				break
			}
			output.Earlier = append(output.Earlier, point.Value)
		}
		value.Outputs = append(value.Outputs, output)
	}
	return value, nil
}
