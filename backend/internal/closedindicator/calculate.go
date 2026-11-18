package closedindicator

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// Output is one named output at the latest stored closed candle. Earlier
// holds its values at the closed candles before it, newest first, as far as
// the requested points reach and the history is continuous. Empty values of
// a sparse output are kept (indicator.Empty), so the values behind them stay
// readable.
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

// Shown returns the value at the latest closed candle as people and JSON
// get it: an empty value is absent.
func (output Output) Shown() (float64, bool) {
	return output.Value, !indicator.Empty(output.Value)
}

// Value holds the named outputs at the latest stored closed candle. Outputs is
// empty when the continuous history ending at that candle is too short.
type Value struct {
	Target   indicator.Target
	OpenTime time.Time
	Outputs  []Output
}

func (value Value) equal(other Value) bool {
	return value.OpenTime.Equal(other.OpenTime) && slices.EqualFunc(value.Outputs, other.Outputs, func(left, right Output) bool {
		return left.Name == right.Name && same(left.Value, right.Value) && slices.EqualFunc(left.Earlier, right.Earlier, same)
	})
}

// same reports equal values; empty values are the same.
func same(left, right float64) bool {
	return left == right || indicator.Empty(left) && indicator.Empty(right)
}

// Depth is the number of closed candles loaded to calculate points values of
// target. Every point gets at least the indicator window of target, the warm-up
// charts give it, so recursive indicators settle and agree with charts. Only
// market.SyncDepth candles are always stored; a deeper depth may load fewer
// candles, so the values it needs stay missing.
func Depth(registry *indicator.Registry, target indicator.Target, points int) (int, error) {
	if !target.Interval().Valid() {
		return 0, fmt.Errorf("closed indicator interval %q is unsupported", target.Interval())
	}
	window, err := registry.Window(target.Selection())
	if err != nil {
		return 0, fmt.Errorf("closed indicator %q: %w", target.Selection().Type, err)
	}
	return window + max(1, points) - 1, nil
}

// calculate evaluates every pair over the closed history of its instrument:
// the in-memory window when it holds every candle the pairs of the history
// need, otherwise the candles read from the store, as deep as the deepest
// pair needs. The pairs whose history cannot be read or whose calculation
// fails are left out, and failed holds their errors.
func (tracker *Tracker) calculate(ctx context.Context, subscriptions []Subscription, inMemory map[historyKey]windowCandles) (values map[pairKey]Value, failed map[pairKey]error) {
	failed = map[pairKey]error{}
	checked := make([]Subscription, 0, len(subscriptions))
	depths := map[historyKey]int{}
	for _, subscription := range subscriptions {
		depth, err := Depth(tracker.registry, subscription.Target, subscription.points())
		if err != nil {
			failed[subscription.pair()] = err
			continue
		}
		subscription.depth = depth
		checked = append(checked, subscription)
		key := subscription.history()
		depths[key] = max(depths[key], depth)
	}
	histories := make(map[historyKey][]market.Candle, len(depths))
	for key, depth := range depths {
		if window, ok := inMemory[key]; ok && window.depth >= depth {
			histories[key] = window.candles
			delete(depths, key)
		}
	}
	read, unread := tracker.readLatest(ctx, depths)
	maps.Copy(histories, read)
	readable := make([]Subscription, 0, len(checked))
	for _, subscription := range checked {
		key := subscription.history()
		if _, ok := depths[key]; ok {
			if err, ok := unread[readGroup{key.interval, depths[key]}]; ok {
				failed[subscription.pair()] = err
				continue
			}
		}
		readable = append(readable, subscription)
	}
	values, failures := tracker.evaluate(readable, histories)
	maps.Copy(failed, failures)
	return values, failed
}

// evaluate calculates every checked pair over the latest candles of its
// history, as deep as the pair needs. The pairs whose calculation fails are
// left out, and failed holds their errors.
func (tracker *Tracker) evaluate(subscriptions []Subscription, histories map[historyKey][]market.Candle) (values map[pairKey]Value, failed map[pairKey]error) {
	values, failed = make(map[pairKey]Value, len(subscriptions)), map[pairKey]error{}
	for _, subscription := range subscriptions {
		candles := histories[subscription.history()]
		value, err := Calculate(tracker.registry, subscription.Target, candles[max(0, len(candles)-subscription.depth):], subscription.points())
		if err != nil {
			failed[subscription.pair()] = err
			continue
		}
		values[subscription.pair()] = value
	}
	return values, failed
}

// Calculate reads the outputs of target at the latest of candles and up to
// points−1 earlier ones. The tracker passes the latest Depth closed candles;
// passing the same window ending at an earlier candle reproduces the value
// the tracker had then.
func Calculate(registry *indicator.Registry, target indicator.Target, candles []market.Candle, points int) (Value, error) {
	return CalculateRange(registry, target, indicator.NewHistory(target.Interval(), candles), 0, len(candles), points)
}

// CalculateRange is Calculate over the window of history, candles of the
// target's interval, from index from through to−1, so a replay prepares its
// history once for the windows of all its candles.
func CalculateRange(registry *indicator.Registry, target indicator.Target, history *indicator.History, from, to, points int) (Value, error) {
	value := Value{Target: target}
	if to <= from {
		return value, nil
	}
	last := history.OpenTime(to - 1).UTC()
	value.OpenTime = last
	outputs, err := registry.CalculateRange(history, from, to, target.Selection())
	if err != nil {
		return Value{}, fmt.Errorf("calculate closed %s: %w", target.Selection().Type, err)
	}
	for _, calculated := range outputs {
		// The values are read from the latest one back, run by run.
		var output Output
		found, expected := false, last
	walk:
		for run := len(calculated.Runs) - 1; run >= 0; run-- {
			series := calculated.Runs[run]
			for index := len(series.Values) - 1; index >= 0; index-- {
				openTime := history.OpenTime(series.Offset + index)
				if !found {
					if !openTime.Equal(last) {
						break walk
					}
					output, found = Output{Name: calculated.Name, Value: series.Values[index]}, true
					continue
				}
				if len(output.Earlier) >= points-1 {
					break walk
				}
				expected = target.Interval().PreviousOpenTime(expected)
				if !openTime.Equal(expected) {
					break walk
				}
				output.Earlier = append(output.Earlier, series.Values[index])
			}
		}
		if found {
			value.Outputs = append(value.Outputs, output)
		}
	}
	return value, nil
}

// readGroup is one store read: the histories of an interval read as deep as
// depth.
type readGroup struct {
	interval market.CandleInterval
	depth    int
}

// readLatest reads the latest candles of each history in depths, as deep as
// its depth, one query per interval and depth, and reports the reads that
// failed, so a history failed only when failed holds
// readGroup{key.interval, depths[key]}.
func (tracker *Tracker) readLatest(ctx context.Context, depths map[historyKey]int) (map[historyKey][]market.Candle, map[readGroup]error) {
	groups := map[readGroup][]int64{}
	for key, depth := range depths {
		at := readGroup{key.interval, depth}
		groups[at] = append(groups[at], key.instrumentID)
	}
	result := make(map[historyKey][]market.Candle, len(depths))
	failed := map[readGroup]error{}
	for at, ids := range groups {
		candles, err := tracker.store.ListLatestCandles(ctx, ids, at.interval, at.depth)
		if err != nil {
			failed[at] = fmt.Errorf("load closed %s history of %d candles: %w", at.interval, at.depth, err)
			continue
		}
		for _, id := range ids {
			result[historyKey{id, at.interval}] = candles[id]
		}
	}
	return result, failed
}
