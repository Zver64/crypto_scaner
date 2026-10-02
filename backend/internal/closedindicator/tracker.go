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

// Equal reports whether both targets identify the same interval and selection.
func (target Target) Equal(other Target) bool { return target.id() == other.id() }

func (target Target) id() string {
	parameters, _ := json.Marshal(target.Selection.Parameters)
	return string(target.Interval) + "|" + string(target.Selection.Type) + "|" + string(parameters)
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

// Demand is a target and the number of latest closed candles whose values
// are kept.
type Demand struct {
	Target Target
	Points int
}

func (subscription Subscription) points() int { return max(1, subscription.Points) }

// Source supplies the pairs that must stay current without any clients.
// targets are the tracker's table targets; a source may track other ones.
type Source interface {
	Subscriptions(ctx context.Context, targets []Target) ([]Subscription, error)
}

// InstrumentSource tracks every table target for each listed instrument.
type InstrumentSource func(context.Context) ([]int64, error)

func (source InstrumentSource) Subscriptions(ctx context.Context, targets []Target) ([]Subscription, error) {
	demands := make([]Demand, len(targets))
	for index, target := range targets {
		demands[index] = Demand{Target: target, Points: 1}
	}
	return source.Track(ctx, demands)
}

// Track tracks every demand for each listed instrument.
func (source InstrumentSource) Track(ctx context.Context, demands []Demand) ([]Subscription, error) {
	ids, err := source(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Subscription, 0, len(ids)*len(demands))
	for _, id := range ids {
		for _, demand := range demands {
			result = append(result, Subscription{InstrumentID: id, Target: demand.Target, Points: demand.Points})
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
	sources  []Source
	logger   *slog.Logger
	wake     chan struct{}
	// retries are applied by Run after retryDelay; only Run's goroutine
	// touches them.
	retries []func()

	mu        sync.Mutex
	targets   []Target
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
		if _, err := tracker.depth(target, 1); err != nil {
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

// SetTargets replaces the table targets, drops cached values of removed ones,
// and reloads the tracked pairs.
func (tracker *Tracker) SetTargets(targets []Target) error {
	for _, target := range targets {
		if _, err := tracker.depth(target, 1); err != nil {
			return err
		}
	}
	kept := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		kept[target.id()] = struct{}{}
	}
	tracker.mu.Lock()
	tracker.targets = slices.Clone(targets)
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
	targets := tracker.targets
	for _, id := range instrumentIDs {
		for _, target := range targets {
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
		// A value calculated before a history change is stale; a newer
		// cached value or none is returned instead.
		if tracker.versions[history] != versions[history] {
			delete(computed, key)
			continue
		}
		if _, exists := tracker.values[key]; !failed && !exists {
			tracker.values[key] = computed[key]
		}
	}
	for _, id := range instrumentIDs {
		values := make([]Value, len(targets))
		for index, target := range targets {
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

// Snapshot returns the known values of the demanded targets for each
// instrument, in demand order, without calculating anything. A pair that is
// neither tracked nor cached has no outputs, and its target is listed in
// missing, so results that depend on it are not final yet; so is a target
// whose value was calculated for fewer points than demanded.
func (tracker *Tracker) Snapshot(instrumentIDs []int64, demands []Demand) (values map[int64][]Value, missing []Target) {
	keys := make([]string, len(demands))
	for index, demand := range demands {
		keys[index] = demand.Target.id()
	}
	values = make(map[int64][]Value, len(instrumentIDs))
	absent := make([]bool, len(demands))
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for _, id := range instrumentIDs {
		row := make([]Value, len(demands))
		for index, demand := range demands {
			value, ok := tracker.values[pairKey{id, keys[index]}]
			if !ok {
				value = Value{Target: demand.Target}
				absent[index] = true
			} else if value.points < demand.Points {
				absent[index] = true
			}
			row[index] = value
		}
		values[id] = row
	}
	for index, demand := range demands {
		if absent[index] {
			missing = append(missing, demand.Target)
		}
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
			if _, err := tracker.depth(subscription.Target, subscription.points()); err != nil {
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

func (tracker *Tracker) depth(target Target, points int) (int, error) {
	return Depth(tracker.registry, Demand{Target: target, Points: points})
}

// Depth is the number of closed candles loaded to calculate the demanded
// points. Every point gets the warm-up of the latest one, so unstable
// indicators agree with charts on all of them. A depth beyond
// market.HistoryDepth only loads the kept candles, so the values it needs
// stay missing.
func Depth(registry *indicator.Registry, demand Demand) (int, error) {
	target := demand.Target
	if !target.Interval.Valid() {
		return 0, fmt.Errorf("closed indicator interval %q is unsupported", target.Interval)
	}
	lookback, err := registry.Lookback(target.Selection.Type, target.Selection.Parameters)
	if err != nil {
		return 0, fmt.Errorf("closed indicator %q: %w", target.Selection.Type, err)
	}
	return max(minimumHistory, lookback+1) + max(1, demand.Points) - 1, nil
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
		points := 1
		for _, subscription := range group {
			points = max(points, subscription.points())
		}
		depth, err := tracker.depth(target, points)
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
		for index, instrumentID := range ids {
			value, err := tracker.value(target, candles[instrumentID], group[index].points())
			if err != nil {
				return nil, err
			}
			result[pairKey{instrumentID, id}] = value
		}
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
