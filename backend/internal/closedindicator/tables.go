package closedindicator

import (
	"context"
	"slices"

	"crypto-scanner/internal/market"
)

func keysByInterval(targets []Target) map[market.CandleInterval][]string {
	result := map[market.CandleInterval][]string{}
	for _, target := range targets {
		result[target.Interval] = append(result[target.Interval], target.Key())
	}
	return result
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
	tracker.forgetFailures()
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

// Latest returns the table targets for each instrument, in target order. A
// value that cannot be loaded has no outputs, so a table shows it as missing;
// the other values are served and cached all the same. A failing pair is
// warned about once until it succeeds.
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
			if _, ok := tracker.values[tablePair(id, keys[index])]; !ok {
				missing = append(missing, Subscription{InstrumentID: id, Target: target})
			}
		}
	}
	versions := tracker.versionSnapshot(missing)
	tracker.mu.Unlock()
	computed, failed := map[pairKey]Value{}, map[pairKey]error{}
	if len(missing) > 0 {
		computed, failed = tracker.calculate(ctx, missing)
	}
	// The caller left, so its reads failed on its context, not on the pairs.
	if ctx.Err() != nil {
		failed = nil
	}
	tracker.mu.Lock()
	failures := tracker.settle(computed, failed)
	for _, subscription := range missing {
		key := subscription.pair()
		history := subscription.history()
		// A value calculated before a history change is stale; a newer
		// cached value or none is returned instead.
		if tracker.versions[history] != versions[history] {
			delete(computed, key)
			continue
		}
		// Only values of current targets are cached, so history changes
		// expire every cached value.
		value, ok := computed[key]
		if _, exists := tracker.values[key]; ok && !exists && tracker.isTablePair(key) {
			tracker.values[key] = value
		}
	}
	for _, id := range instrumentIDs {
		values := make([]Value, len(targets))
		for index, target := range targets {
			key := tablePair(id, keys[index])
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
	tracker.mu.Unlock()
	tracker.warn(ctx, "load closed indicators failed", failures)
	return result
}

// versionSnapshot must be called with the lock held.
func (tracker *Tracker) versionSnapshot(subscriptions []Subscription) map[historyKey]uint64 {
	result := make(map[historyKey]uint64, len(subscriptions))
	for _, subscription := range subscriptions {
		key := subscription.history()
		result[key] = tracker.versions[key]
	}
	return result
}
