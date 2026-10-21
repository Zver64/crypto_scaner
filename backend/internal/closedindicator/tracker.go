// Package closedindicator keeps indicator values at the latest closed candle
// current in the background and serves the same values to tables.
package closedindicator

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
)

// Change reports a background recalculation of a tracked pair whose value
// differs from the last known one. Previous is zero when nothing was known.
type Change struct {
	InstrumentID int64
	Previous     Value
	Current      Value
}

// Subscription requests background tracking of one target for one instrument.
// Points is the number of latest closed candles whose values are kept; zero
// keeps only the latest one. Points is part of its identity: subscriptions of
// one target with different points are calculated separately, each over its
// own depth, while identical ones share one value.
type Subscription struct {
	InstrumentID int64
	Target       Target
	Points       int
	// depth is the number of candles the subscription needs, set once it is
	// checked.
	depth int
}

func (subscription Subscription) points() int { return max(1, subscription.Points) }

func (subscription Subscription) pair() pairKey {
	return pairKey{subscription.InstrumentID, subscription.Target.Key(), subscription.points()}
}

func (subscription Subscription) history() historyKey {
	return historyKey{subscription.InstrumentID, subscription.Target.Interval}
}

type Store interface {
	// ListLatestCandles returns up to limit latest closed candles per
	// instrument in chronological order.
	ListLatestCandles(context.Context, []int64, market.CandleInterval, int) (map[int64][]market.Candle, error)
}

// pairKey identifies a subscription; table values keep one point.
type pairKey struct {
	instrumentID int64
	target       string
	points       int
}

// tablePair identifies the table value of the target with key target.
func tablePair(instrumentID int64, target string) pairKey {
	return pairKey{instrumentID, target, 1}
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
	// failing holds the tracked and table pairs whose latest calculation
	// failed; a pair is warned about again only after it succeeded.
	failing *logging.Failures[pairKey]
	// invalid holds the invalid targets of each subscriber, warned about
	// when they become invalid.
	invalid *logging.Failures[invalidTarget]
	// loads holds the intervals whose latest window load failed.
	loads *logging.Failures[market.CandleInterval]
}

// invalidTarget is a target invalid for a subscriber.
type invalidTarget struct {
	subscriber, target string
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
		versions: map[historyKey]uint64{}, synced: map[market.CandleInterval]struct{}{}, failing: logging.NewFailures[pairKey](logger), invalid: logging.NewFailures[invalidTarget](logger),
		loads: logging.NewFailures[market.CandleInterval](logger),
	}
	for _, target := range targets {
		if _, err := Depth(registry, target, 1); err != nil {
			return nil, err
		}
	}
	return tracker, nil
}

// Listen sets the consumer of recalculated tracked values, replacing any
// earlier one. It receives the changes of each step at once, such as those of
// a synchronization round with the windows it loaded, outside the tracker
// lock, and must not block.
func (tracker *Tracker) Listen(listener func([]Change)) {
	tracker.mu.Lock()
	tracker.listener = listener
	tracker.mu.Unlock()
}

// Subscribe replaces the pairs subscriber keeps current, which the tracker
// calculates at once and keeps current in the background, together with the
// pairs of the other subscribers; the pairs nobody keeps any more are
// dropped. Invalid pairs, and pairs deeper than the synchronized history,
// whose windows committed changes would not keep current, are left out; a
// target is warned about when it becomes invalid for subscriber. It never
// blocks.
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
	tracker.mu.Lock()
	tracker.subscribers[subscriber] = valid
	tracker.refresh = true
	tracker.mu.Unlock()
	tracker.signal()
	tracker.invalid.Retain(func(key invalidTarget) bool {
		_, still := invalid[key.target]
		return key.subscriber != subscriber || still
	})
	for _, key := range slices.Sorted(maps.Keys(invalid)) {
		tracker.invalid.Warn(context.Background(), invalidTarget{subscriber, key}, "skip invalid closed indicator subscription", "module", "closed_indicator", "subscriber", subscriber, "target", key, "error", invalid[key])
	}
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
			pair := tablePair(key.instrumentID, target)
			if _, tracked := tracker.tracked[pair]; !tracked {
				delete(tracker.values, pair)
			}
		}
	}
}

// HistorySynced recalculates the tracked pairs of interval whose history
// changed, once a synchronization round of interval has committed all its
// changes. A failed load is retried by the next step, such as one a round, a
// subscription change, or a history load starts; a failed calculation by the
// next round of its interval, which always starts one. It never blocks.
func (tracker *Tracker) HistorySynced(interval market.CandleInterval) {
	tracker.mu.Lock()
	dirty, unloaded := tracker.windows.pendingIn(interval)
	if dirty {
		tracker.synced[interval] = struct{}{}
	}
	tracker.mu.Unlock()
	if dirty || unloaded {
		tracker.signal()
	}
}

// Snapshot returns the known value of each subscription, in subscription
// order, without calculating anything. A pair that is neither tracked nor
// cached has no outputs, and its target is listed in missing, so results that
// depend on it are not final yet.
func (tracker *Tracker) Snapshot(subscriptions []Subscription) (values []Value, missing []Target) {
	values = make([]Value, len(subscriptions))
	absent := map[string]Target{}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for index, subscription := range subscriptions {
		value, ok := tracker.values[subscription.pair()]
		if !ok {
			value = Value{Target: subscription.Target}
			absent[subscription.Target.Key()] = subscription.Target
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tracker.wake:
			tracker.step(ctx)
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
// synchronized histories, and those whose windows were missing or too short,
// after reading them. All of them are calculated at once, so the listener
// receives the changes of a step in one call.
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
	tracker.mu.Unlock()
	if len(loads) > 0 && !tracker.fill(ctx, loads, due) {
		return
	}
	tracker.mu.Lock()
	current := tracker.prepare(due)
	tracker.mu.Unlock()
	tracker.recalculate(ctx, current)
}

// resubscribe tracks the pairs of every subscriber, plans their windows, and
// adds to due the new pairs whose windows are ready. It must be called with
// the lock held.
func (tracker *Tracker) resubscribe(due map[pairKey]Subscription) {
	next := map[pairKey]Subscription{}
	for _, subscriptions := range tracker.subscribers {
		for _, subscription := range subscriptions {
			next[subscription.pair()] = subscription
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
	tracker.forgetFailures()
	tracker.windows.plan(next)
	for key, subscription := range next {
		_, tracked := previous[key]
		ready := tracker.windows[subscription.history()].ready()
		if ready && !tracked {
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
		key := subscription.history()
		result.windows[key] = tracker.windows[key].candles
		result.versions[key] = tracker.versions[key]
	}
	return result
}

// fill reads the windows of loads, installs them with the changes committed
// meanwhile, and adds to due the pairs whose windows became ready. The windows
// of a failed interval stay unloaded, so the next step loads them again. It
// reports false when ctx is cancelled.
func (tracker *Tracker) fill(ctx context.Context, loads map[historyKey]load, due map[pairKey]Subscription) bool {
	depths := make(map[historyKey]int, len(loads))
	for key, started := range loads {
		depths[key] = started.depth
	}
	histories, failed := tracker.readLatest(ctx, depths)
	if ctx.Err() != nil {
		return false
	}
	failing := map[market.CandleInterval]struct{}{}
	for at, err := range failed {
		failing[at.interval] = struct{}{}
		tracker.loads.Warn(ctx, at.interval, "load closed indicator history failed", "module", "closed_indicator", "interval", at.interval, "error", err)
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for key, started := range loads {
		if _, ok := failing[key.interval]; !ok {
			tracker.loads.Forget(key.interval)
		}
		if _, ok := failed[readGroup{key.interval, started.depth}]; ok {
			tracker.windows.fail(key, started)
		} else if tracker.windows.install(key, started, histories[key]) {
			tracker.addPairs(due, key)
		}
	}
	return true
}

// recalculate evaluates the pairs of current and publishes the values that
// changed; a value calculated before a newer change of its history is left
// to that change's recalculation. A pair whose calculation fails marks its
// history dirty, so the next round of its interval recalculates it, and is
// warned about once until it succeeds.
func (tracker *Tracker) recalculate(ctx context.Context, current calculation) {
	if len(current.due) == 0 {
		return
	}
	computed, failed := tracker.evaluate(slices.Collect(maps.Values(current.due)), current.windows)
	var changes []Change
	tracker.mu.Lock()
	maps.DeleteFunc(failed, func(key pairKey, _ error) bool {
		_, tracked := tracker.tracked[key]
		return !tracked
	})
	failures := tracker.settle(computed, failed)
	for key, subscription := range current.due {
		at := subscription.history()
		if _, tracked := tracker.tracked[key]; !tracked {
			delete(tracker.values, key)
			continue
		}
		value, ok := computed[key]
		if !ok {
			if window := tracker.windows[at]; window != nil {
				window.dirty = true
			}
			continue
		}
		if tracker.versions[at] != current.versions[at] {
			continue
		}
		previous, existed := tracker.values[key]
		tracker.values[key] = value
		if !existed || !previous.equal(value) {
			changes = append(changes, Change{InstrumentID: subscription.InstrumentID, Previous: previous, Current: value})
		}
	}
	listener := tracker.listener
	tracker.mu.Unlock()
	tracker.warn(ctx, "recalculate closed indicator failed", failures)
	if listener != nil && len(changes) > 0 {
		listener(changes)
	}
}

// failure is an error of the failing pairs of one target, their number, and
// the number of those that newly fail.
type failure struct {
	err                error
	instruments, fresh int
}

// settle forgets the failures of the pairs in succeeded and records those in
// failed. It returns the failures by target, counting the pairs that were
// not failing yet, so a failing pair is warned about once until it succeeds.
// It must be called with the lock held.
func (tracker *Tracker) settle(succeeded map[pairKey]Value, failed map[pairKey]error) map[string]failure {
	for key := range succeeded {
		tracker.failing.Forget(key)
	}
	failures := map[string]failure{}
	for key, err := range failed {
		current := failures[key.target]
		if current.err == nil {
			current.err = err
		}
		if tracker.failing.Mark(key) {
			current.fresh++
		}
		current.instruments++
		failures[key.target] = current
	}
	return failures
}

// warn logs one record per target of failures: a warning when some of its
// pairs newly fail, otherwise an INFO record.
func (tracker *Tracker) warn(ctx context.Context, message string, failures map[string]failure) {
	for _, target := range slices.Sorted(maps.Keys(failures)) {
		failure := failures[target]
		level := slog.LevelInfo
		if failure.fresh > 0 {
			level = slog.LevelWarn
		}
		tracker.logger.Log(ctx, level, message, "module", "closed_indicator", "target", target, "instruments", failure.instruments, "error", failure.err)
	}
}

// forgetFailures drops the failing pairs that are neither tracked nor table
// pairs. It must be called with the lock held.
func (tracker *Tracker) forgetFailures() {
	tracker.failing.Retain(func(key pairKey) bool {
		_, tracked := tracker.tracked[key]
		return tracked || tracker.isTablePair(key)
	})
}

// isTablePair reports whether key is the table value of a current table
// target. It must be called with the lock held.
func (tracker *Tracker) isTablePair(key pairKey) bool {
	if key.points != 1 {
		return false
	}
	for _, keys := range tracker.tableKeys {
		if slices.Contains(keys, key.target) {
			return true
		}
	}
	return false
}
