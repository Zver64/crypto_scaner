package closedindicator

import (
	"slices"

	"crypto-scanner/internal/market"
)

// window is one tracked history, instrument × interval: the tracked pairs
// that read it and its latest closed candles in memory.
type window struct {
	pairs []pairKey
	// depth is the number of candles the deepest of pairs needs.
	depth int
	// candles is the window, chronological, as deep as loaded; it is never
	// modified in place, so a reader may keep it outside the lock.
	candles []market.Candle
	// loaded is the depth candles were loaded for, 0 before the first load.
	loaded int
	// loading marks a running load, which changes committed meanwhile may
	// miss, so changes buffers them; failed holds a failed load back until
	// the retry.
	loading bool
	changes []market.Candle
	failed  bool
	// dirty marks changes since the pairs were last calculated.
	dirty bool
}

// ready reports whether the window holds every candle the pairs need; a
// window is never ready before its first load.
func (current *window) ready() bool { return current.loaded > 0 && current.loaded >= current.depth }

// windows are the histories of the tracked pairs. Their methods must be
// called with the tracker lock held.
type windows map[historyKey]*window

// plan follows the tracked pairs, with the depth each needs: a history no
// pair reads any more is freed, a shallower one is trimmed, and a deeper or
// new one awaits its load.
func (all windows) plan(tracked map[pairKey]Subscription) {
	for _, current := range all {
		current.pairs, current.depth = nil, 0
	}
	for key, subscription := range tracked {
		at := historyKey{subscription.InstrumentID, subscription.Target.Interval}
		current := all[at]
		if current == nil {
			current = &window{}
			all[at] = current
		}
		current.pairs = append(current.pairs, key)
		current.depth = max(current.depth, subscription.depth)
	}
	for key, current := range all {
		switch {
		case len(current.pairs) == 0:
			delete(all, key)
		case current.ready() && current.loaded > current.depth:
			current.candles = latest(current.candles, current.depth)
			current.loaded = current.depth
		}
	}
}

// apply merges committed changes into the windows and buffers them for the
// running loads; it marks the histories dirty.
func (all windows) apply(changes map[historyKey][]market.Candle) {
	for key, candles := range changes {
		current := all[key]
		if current == nil {
			continue
		}
		if current.loaded > 0 {
			current.candles = merged(current.candles, candles, current.loaded)
		}
		if current.loading {
			current.changes = append(current.changes, candles...)
		}
		current.dirty = true
	}
}

// load is a started load of the window of one history.
type load struct {
	window *window
	depth  int
}

// startLoads marks the windows that must be loaded as loading and returns
// their loads, leaving failed ones for the retry.
func (all windows) startLoads() map[historyKey]load {
	loads := map[historyKey]load{}
	for key, current := range all {
		if current.ready() || current.loading || current.failed {
			continue
		}
		current.loading, current.changes = true, nil
		loads[key] = load{current, current.depth}
	}
	return loads
}

// install completes started with the candles read and the changes committed
// while it ran, and reports whether the window is ready. A load of a window
// no longer tracked, or replaced since, is left out.
func (all windows) install(key historyKey, started load, candles []market.Candle) bool {
	current := all[key]
	if current != started.window || !current.loading {
		return false
	}
	current.candles = merged(candles, current.changes, started.depth)
	current.loaded, current.loading, current.changes, current.dirty = started.depth, false, nil, false
	return current.ready()
}

// fail ends started without a window; it waits for retryFailed.
func (all windows) fail(key historyKey, started load) {
	if current := all[key]; current == started.window && current.loading {
		current.loading, current.failed, current.changes = false, true, nil
	}
}

// retryFailed lets the failed loads run again.
func (all windows) retryFailed() {
	for _, current := range all {
		current.failed = false
	}
}

// takeDirty returns the ready dirty histories of the synced intervals and
// clears their marks; the others wait for their own round or load.
func (all windows) takeDirty(synced map[market.CandleInterval]struct{}) []historyKey {
	var keys []historyKey
	for key, current := range all {
		if _, ok := synced[key.interval]; ok && current.dirty && current.ready() {
			current.dirty = false
			keys = append(keys, key)
		}
	}
	return keys
}

// dirtyIn reports whether any ready window of interval is dirty; the others
// recalculate after their loads.
func (all windows) dirtyIn(interval market.CandleInterval) bool {
	for key, current := range all {
		if key.interval == interval && current.dirty && current.ready() {
			return true
		}
	}
	return false
}

// merged returns window with changes applied by open time, a change
// replacing the stored candle of its open time and later changes earlier
// ones, keeping the latest depth. Neither input is modified.
func merged(window, changes []market.Candle, depth int) []market.Candle {
	sorted := slices.SortedStableFunc(slices.Values(changes), func(left, right market.Candle) int {
		return left.OpenTime.Compare(right.OpenTime)
	})
	result := make([]market.Candle, 0, len(window)+len(sorted))
	put := func(candle market.Candle) {
		if last := len(result) - 1; last >= 0 && result[last].OpenTime.Equal(candle.OpenTime) {
			result[last] = candle
			return
		}
		result = append(result, candle)
	}
	index := 0
	for _, change := range sorted {
		for ; index < len(window) && window[index].OpenTime.Before(change.OpenTime); index++ {
			put(window[index])
		}
		if index < len(window) && window[index].OpenTime.Equal(change.OpenTime) {
			index++
		}
		put(change)
	}
	for _, candle := range window[index:] {
		put(candle)
	}
	// result is fresh, so trimming it needs no copy.
	return result[max(0, len(result)-depth):]
}

// latest returns a copy of the latest depth candles, so a shorter window
// does not keep a longer array.
func latest(candles []market.Candle, depth int) []market.Candle {
	return slices.Clone(candles[max(0, len(candles)-depth):])
}
