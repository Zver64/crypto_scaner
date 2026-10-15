// Package closedindicator keeps indicator values at the latest closed candle
// current in the background and serves the same values to tables.
package closedindicator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
)

const retryDelay = 30 * time.Second

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
	// depth is the number of candles the subscription needs, set once it is
	// checked.
	depth int
}

func (subscription Subscription) points() int { return max(1, subscription.Points) }

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

// Tracker recalculates tracked pairs over in-memory windows of their closed
// history once a synchronization round changed it, and caches on-demand table
// values until the history of their instrument changes.
type Tracker struct {
	store    Store
	registry *indicator.Registry
	logger   *slog.Logger
	wake     chan struct{}
	// retries are applied by Run after retryDelay; only Run's goroutine
	// touches them.
	retries []func()

	mu sync.Mutex
	// subscribers hold the pairs each subscriber keeps current.
	subscribers map[string][]Subscription
	targets     []Target
	// tableKeys holds the keys of targets by interval; every cached value
	// that is not tracked belongs to one of them.
	tableKeys map[market.CandleInterval][]string
	tracked   map[pairKey]Subscription
	// windows hold the candles of the histories of the tracked pairs, which
	// committed changes keep current without reading the store.
	windows  windows
	values   map[pairKey]Value
	versions map[historyKey]uint64
	// synced are the intervals whose dirty histories the next step
	// recalculates; the others wait for the end of their own
	// synchronization round, so a round is recalculated at once.
	synced   map[market.CandleInterval]struct{}
	refresh  bool
	listener func([]Change)
}

// New creates a tracker. Targets are the values served to tables; Subscribe
// decides which pairs are kept current in the background.
func New(store Store, registry *indicator.Registry, targets []Target, logger *slog.Logger) (*Tracker, error) {
	if store == nil || registry == nil || logger == nil {
		return nil, fmt.Errorf("closed indicator store, indicator registry, and logger are required")
	}
	tracker := &Tracker{
		store: store, registry: registry, targets: targets, tableKeys: keysByInterval(targets), logger: logger, subscribers: map[string][]Subscription{},
		wake: make(chan struct{}, 1), tracked: map[pairKey]Subscription{}, windows: windows{}, values: map[pairKey]Value{},
		versions: map[historyKey]uint64{}, synced: map[market.CandleInterval]struct{}{},
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

// Listen sets the consumer of recalculated tracked values, replacing any
// earlier one. It receives the changes of each recalculation at once, such as
// those of a synchronization round, outside the tracker lock, and must not
// block.
func (tracker *Tracker) Listen(listener func([]Change)) {
	tracker.mu.Lock()
	tracker.listener = listener
	tracker.mu.Unlock()
}

// SetTargets replaces the table targets and drops cached values of removed
// ones.
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
	tracker.mu.Unlock()
	return nil
}

// Subscribe replaces the pairs subscriber keeps current, which the tracker
// calculates at once and keeps current in the background, together with the
// pairs of the other subscribers; the pairs nobody keeps any more are
// dropped. Invalid pairs, and pairs deeper than the synchronized history,
// whose windows committed changes would not keep current, are left out. It
// never blocks.
func (tracker *Tracker) Subscribe(subscriber string, subscriptions []Subscription) {
	valid := make([]Subscription, 0, len(subscriptions))
	// One warning names each invalid target, however many coins read it.
	invalid := map[string]error{}
	for _, subscription := range subscriptions {
		depth, err := Depth(tracker.registry, subscription.Target, subscription.points())
		if err == nil && depth > market.SyncDepth {
			err = fmt.Errorf("needs %d closed candles, %d are synchronized", depth, market.SyncDepth)
		}
		if err != nil {
			invalid[subscription.Target.Key()] = err
			continue
		}
		subscription.depth = depth
		valid = append(valid, subscription)
	}
	for _, key := range slices.Sorted(maps.Keys(invalid)) {
		tracker.logger.Warn("skip invalid closed indicator subscription", "module", "closed_indicator", "subscriber", subscriber, "target", key, "error", invalid[key])
	}
	tracker.mu.Lock()
	tracker.subscribers[subscriber] = valid
	tracker.refresh = true
	tracker.mu.Unlock()
	tracker.signal()
}

// HistoryChanged applies committed closed-candle changes to the windows,
// invalidates values, and marks tracked histories for the recalculation
// HistorySynced starts. It never waits for recalculation.
func (tracker *Tracker) HistoryChanged(candles []market.Candle) {
	touched := map[historyKey][]market.Candle{}
	for _, candle := range candles {
		key := historyKey{candle.InstrumentID, candle.Interval}
		touched[key] = append(touched[key], candle)
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.windows.apply(touched)
	for key := range touched {
		tracker.versions[key]++
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

// HistorySynced recalculates the tracked pairs of interval whose history
// changed, once a synchronization round of interval has committed all its
// changes. It never blocks.
func (tracker *Tracker) HistorySynced(interval market.CandleInterval) {
	tracker.mu.Lock()
	marked := tracker.windows.dirtyIn(interval)
	if marked {
		tracker.synced[interval] = struct{}{}
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
	retry := time.NewTimer(retryDelay)
	retry.Stop()
	defer retry.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
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

// step recalculates the pairs that are due: the new ones, those of
// synchronized histories, and, after reading their windows, those whose
// windows were missing or too short. Ready pairs never wait for loads.
func (tracker *Tracker) step(ctx context.Context) {
	tracker.mu.Lock()
	due := map[pairKey]Subscription{}
	if tracker.refresh {
		tracker.refresh = false
		tracker.resubscribe(due)
	}
	for _, key := range tracker.windows.takeDirty(tracker.synced) {
		tracker.addPairs(due, key)
	}
	clear(tracker.synced)
	loads := tracker.windows.startLoads()
	ready := tracker.prepare(due)
	tracker.mu.Unlock()
	tracker.recalculate(ctx, ready)
	if len(loads) > 0 {
		tracker.recalculate(ctx, tracker.fill(ctx, loads))
	}
}

// resubscribe tracks the pairs of every subscriber, plans their windows, and
// adds to due the new pairs and those that now keep more points whose
// windows are ready. It must be called with the lock held.
func (tracker *Tracker) resubscribe(due map[pairKey]Subscription) {
	next := map[pairKey]Subscription{}
	for _, subscriptions := range tracker.subscribers {
		for _, subscription := range subscriptions {
			key := pairKey{subscription.InstrumentID, subscription.Target.Key()}
			if existing, ok := next[key]; ok {
				subscription.Points = max(subscription.points(), existing.points())
				subscription.depth = max(subscription.depth, existing.depth)
			}
			next[key] = subscription
		}
	}
	// A dropped pair may hold a value awaiting recalculation, which would
	// otherwise be served as a fresh cache entry.
	for key := range tracker.tracked {
		if _, kept := next[key]; !kept {
			delete(tracker.values, key)
		}
	}
	previous := tracker.tracked
	tracker.tracked = next
	tracker.windows.plan(next)
	for key, subscription := range next {
		existing, tracked := previous[key]
		ready := tracker.windows[historyKey{subscription.InstrumentID, subscription.Target.Interval}].ready()
		if ready && (!tracked || existing.points() != subscription.points()) {
			due[key] = subscription
		}
	}
}

// addPairs adds the tracked pairs of history to due. It must be called with
// the lock held.
func (tracker *Tracker) addPairs(due map[pairKey]Subscription, history historyKey) {
	for _, key := range tracker.windows[history].pairs {
		due[key] = tracker.tracked[key]
	}
}

// calculation is a set of due pairs with the windows and history versions
// they are calculated from.
type calculation struct {
	due      map[pairKey]Subscription
	windows  map[historyKey][]market.Candle
	versions map[historyKey]uint64
}

// prepare takes the windows and versions of due. It must be called with the
// lock held; windows are never modified in place, so they need no copy.
func (tracker *Tracker) prepare(due map[pairKey]Subscription) calculation {
	result := calculation{due: due, windows: map[historyKey][]market.Candle{}, versions: map[historyKey]uint64{}}
	for _, subscription := range due {
		key := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		result.windows[key] = tracker.windows[key].candles
		result.versions[key] = tracker.versions[key]
	}
	return result
}

// fill reads the windows of loads, installs them with the changes committed
// meanwhile, and returns the calculation of the pairs whose windows became
// ready. The loads of a failed interval wait for the retry.
func (tracker *Tracker) fill(ctx context.Context, loads map[historyKey]load) calculation {
	depths := make(map[historyKey]int, len(loads))
	for key, started := range loads {
		depths[key] = started.depth
	}
	histories, failed := tracker.readLatest(ctx, depths)
	if ctx.Err() != nil {
		return calculation{}
	}
	for interval, err := range failed {
		tracker.logger.WarnContext(ctx, "load closed indicator history failed", "module", "closed_indicator", "interval", interval, "error", err)
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(failed) > 0 {
		tracker.retry(tracker.windows.retryFailed)
	}
	due := map[pairKey]Subscription{}
	for key, started := range loads {
		if _, ok := failed[key.interval]; ok {
			tracker.windows.fail(key, started)
		} else if tracker.windows.install(key, started, histories[key]) {
			tracker.addPairs(due, key)
		}
	}
	return tracker.prepare(due)
}

// recalculate evaluates the pairs of current and publishes the values that
// changed; a value calculated before a newer change of its history is left
// to that change's recalculation.
func (tracker *Tracker) recalculate(ctx context.Context, current calculation) {
	if len(current.due) == 0 {
		return
	}
	computed, err := tracker.evaluate(slices.Collect(maps.Values(current.due)), current.windows)
	if err != nil {
		tracker.logger.WarnContext(ctx, "recalculate closed indicators failed", "module", "closed_indicator", "error", err)
		tracker.retry(func() {
			for key := range current.windows {
				if window := tracker.windows[key]; window != nil {
					window.dirty = true
					tracker.synced[key.interval] = struct{}{}
				}
			}
		})
		return
	}
	var changes []Change
	tracker.mu.Lock()
	for key, subscription := range current.due {
		at := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		if _, tracked := tracker.tracked[key]; !tracked {
			delete(tracker.values, key)
			continue
		}
		if tracker.versions[at] != current.versions[at] {
			continue
		}
		previous, existed := tracker.values[key]
		value := computed[key]
		tracker.values[key] = value
		if !existed || !previous.equal(value) {
			changes = append(changes, Change{InstrumentID: subscription.InstrumentID, Previous: previous, Current: value})
		}
	}
	listener := tracker.listener
	tracker.mu.Unlock()
	if listener != nil && len(changes) > 0 {
		listener(changes)
	}
}

// readLatest reads the latest candles of each history in depths, as deep as
// its depth, one query per interval and depth, and reports the intervals
// that failed.
func (tracker *Tracker) readLatest(ctx context.Context, depths map[historyKey]int) (map[historyKey][]market.Candle, map[market.CandleInterval]error) {
	type group struct {
		interval market.CandleInterval
		depth    int
	}
	groups := map[group][]int64{}
	for key, depth := range depths {
		at := group{key.interval, depth}
		groups[at] = append(groups[at], key.instrumentID)
	}
	result := make(map[historyKey][]market.Candle, len(depths))
	failed := map[market.CandleInterval]error{}
	for at, ids := range groups {
		candles, err := tracker.store.ListLatestCandles(ctx, ids, at.interval, at.depth)
		if err != nil {
			failed[at.interval] = fmt.Errorf("load closed %s history: %w", at.interval, err)
			continue
		}
		for _, id := range ids {
			result[historyKey{id, at.interval}] = candles[id]
		}
	}
	return result, failed
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
// target. Every point gets at least the chart.Window of target, the warm-up
// charts give it, so recursive indicators settle and agree with charts. Only
// market.SyncDepth candles are always stored; a deeper depth may load fewer
// candles, so the values it needs stay missing.
func Depth(registry *indicator.Registry, target Target, points int) (int, error) {
	if !target.Interval.Valid() {
		return 0, fmt.Errorf("closed indicator interval %q is unsupported", target.Interval)
	}
	window, err := chart.Window(registry, target.Selection)
	if err != nil {
		return 0, fmt.Errorf("closed indicator %q: %w", target.Selection.Type, err)
	}
	return window + max(1, points) - 1, nil
}

// calculate reads the closed histories of table values from the store, as
// deep as their deepest pair needs, and evaluates every pair.
func (tracker *Tracker) calculate(ctx context.Context, subscriptions []Subscription) (map[pairKey]Value, error) {
	depths := map[historyKey]int{}
	for index, subscription := range subscriptions {
		depth, err := Depth(tracker.registry, subscription.Target, subscription.points())
		if err != nil {
			return nil, err
		}
		subscriptions[index].depth = depth
		key := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		depths[key] = max(depths[key], depth)
	}
	histories, failed := tracker.readLatest(ctx, depths)
	if len(failed) > 0 {
		return nil, errors.Join(slices.Collect(maps.Values(failed))...)
	}
	return tracker.evaluate(subscriptions, histories)
}

// evaluate calculates every checked pair over the latest candles of its
// history, as deep as the pair needs.
func (tracker *Tracker) evaluate(subscriptions []Subscription, histories map[historyKey][]market.Candle) (map[pairKey]Value, error) {
	result := make(map[pairKey]Value, len(subscriptions))
	for _, subscription := range subscriptions {
		candles := histories[historyKey{subscription.InstrumentID, subscription.Target.Interval}]
		value, err := Calculate(tracker.registry, subscription.Target, candles[max(0, len(candles)-subscription.depth):], subscription.points())
		if err != nil {
			return nil, err
		}
		result[pairKey{subscription.InstrumentID, subscription.Target.Key()}] = value
	}
	return result, nil
}

// Calculate reads the outputs of target at the latest of candles and up to
// points−1 earlier ones. The tracker passes the latest Depth closed candles;
// passing the same window ending at an earlier candle reproduces the value
// the tracker had then.
func Calculate(registry *indicator.Registry, target Target, candles []market.Candle, points int) (Value, error) {
	value := Value{Target: target, points: points}
	if len(candles) == 0 {
		return value, nil
	}
	last := candles[len(candles)-1].OpenTime.UTC()
	value.OpenTime = last
	results, err := registry.CalculateCandles(target.Interval, candles, []indicator.Selection{target.Selection})
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
