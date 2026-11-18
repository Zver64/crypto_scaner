package closedindicator

import (
	"context"
	"slices"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// tableCache holds the table values calculated on demand, each until the
// history of its instrument changes. Its methods must be called with the
// tracker lock held.
type tableCache struct {
	targets []indicator.Target
	// keys holds the keys of targets.
	keys map[string]struct{}
	// values hold the cached values of each history by target key.
	values map[historyKey]map[string]Value
	// versions count the expiries of each history, so a value calculated
	// before one is not cached.
	versions map[historyKey]uint64
}

func newTableCache(targets []indicator.Target) *tableCache {
	cache := &tableCache{values: map[historyKey]map[string]Value{}, versions: map[historyKey]uint64{}}
	cache.setTargets(targets)
	return cache
}

// setTargets replaces the targets and drops the values of removed ones.
func (cache *tableCache) setTargets(targets []indicator.Target) {
	cache.targets = slices.Clone(targets)
	cache.keys = make(map[string]struct{}, len(targets))
	for _, target := range targets {
		cache.keys[target.Key()] = struct{}{}
	}
	for _, values := range cache.values {
		for key := range values {
			if _, kept := cache.keys[key]; !kept {
				delete(values, key)
			}
		}
	}
}

// holds reports whether key is the table value of a current target.
func (cache *tableCache) holds(key pairKey) bool {
	_, ok := cache.keys[key.target]
	return ok && key.points == 1
}

func (cache *tableCache) get(key pairKey) (Value, bool) {
	if key.points != 1 {
		return Value{}, false
	}
	value, ok := cache.values[historyKey{key.instrumentID, key.interval}][key.target]
	return value, ok
}

// expire drops the values of history after it changed.
func (cache *tableCache) expire(history historyKey) {
	cache.versions[history]++
	delete(cache.values, history)
}

// put caches the value of the table pair of subscription unless history
// expired since version or a value is cached already, and reports whether
// value is current.
func (cache *tableCache) put(subscription Subscription, version uint64, value Value) bool {
	history := subscription.history()
	if cache.versions[history] != version {
		return false
	}
	key := subscription.Target.Key()
	if _, kept := cache.keys[key]; !kept {
		return true
	}
	values := cache.values[history]
	if values == nil {
		values = map[string]Value{}
		cache.values[history] = values
	}
	if _, exists := values[key]; !exists {
		values[key] = value
	}
	return true
}

// SetTargets replaces the table targets and drops cached values of removed
// ones.
func (tracker *Tracker) SetTargets(targets []indicator.Target) error {
	for _, target := range targets {
		if _, err := Depth(tracker.registry, target, 1); err != nil {
			return err
		}
	}
	tracker.mu.Lock()
	tracker.table.setTargets(targets)
	tracker.forgetFailures()
	tracker.mu.Unlock()
	return nil
}

// Latest returns the table targets for each instrument, in target order: the
// tracked or cached value, or else one calculated now, over the in-memory
// window of the history when it is deep enough and over stored candles
// otherwise. A value that cannot be loaded has no outputs, so a table shows
// it as missing; the other values are served and cached all the same. A
// failing pair is warned about once until it succeeds.
func (tracker *Tracker) Latest(ctx context.Context, instrumentIDs []int64) map[int64][]Value {
	result := make(map[int64][]Value, len(instrumentIDs))
	var missing []Subscription
	tracker.mu.Lock()
	targets := tracker.table.targets
	for _, id := range instrumentIDs {
		for _, target := range targets {
			subscription := Subscription{InstrumentID: id, Target: target}
			if _, ok := tracker.known(subscription.pair()); !ok {
				missing = append(missing, subscription)
			}
		}
	}
	versions := make(map[historyKey]uint64, len(missing))
	inMemory := map[historyKey]windowCandles{}
	for _, subscription := range missing {
		key := subscription.history()
		versions[key] = tracker.table.versions[key]
		if current := tracker.windows[key]; current != nil && current.loaded > 0 {
			inMemory[key] = windowCandles{current.candles, current.loaded}
		}
	}
	tracker.mu.Unlock()
	computed, failed := map[pairKey]Value{}, map[pairKey]error{}
	if len(missing) > 0 {
		computed, failed = tracker.calculate(ctx, missing, inMemory)
	}
	// The caller left, so its reads failed on its context, not on the pairs.
	if ctx.Err() != nil {
		failed = nil
	}
	tracker.mu.Lock()
	failures := tracker.settle(computed, failed)
	for _, subscription := range missing {
		key := subscription.pair()
		value, ok := computed[key]
		if !ok {
			continue
		}
		// A value calculated before a history change is stale; a newer
		// known value or none is returned instead.
		if !tracker.table.put(subscription, versions[subscription.history()], value) {
			delete(computed, key)
		}
	}
	for _, id := range instrumentIDs {
		values := make([]Value, len(targets))
		for index, target := range targets {
			key := pairKey{id, target.Interval(), target.Key(), 1}
			if value, ok := computed[key]; ok {
				values[index] = value
			} else if value, ok := tracker.known(key); ok {
				values[index] = value
			} else {
				values[index] = Value{Target: target}
			}
		}
		result[id] = values
	}
	tracker.mu.Unlock()
	tracker.warn(ctx, "load closed indicators failed", failures)
	return result
}

// windowCandles are the candles of an in-memory window, the latest depth
// closed candles of their history.
type windowCandles struct {
	candles []market.Candle
	depth   int
}
