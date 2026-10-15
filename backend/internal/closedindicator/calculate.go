package closedindicator

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
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
	values, failures := tracker.evaluate(subscriptions, histories)
	if len(failures) > 0 {
		errs := make([]error, 0, len(failures))
		for _, key := range slices.Sorted(maps.Keys(failures)) {
			errs = append(errs, failures[key])
		}
		return nil, errors.Join(errs...)
	}
	return values, nil
}

// evaluate calculates every checked pair over the latest candles of its
// history, as deep as the pair needs. The pairs whose calculation fails are
// left out, and failed holds one error per target key of them.
func (tracker *Tracker) evaluate(subscriptions []Subscription, histories map[historyKey][]market.Candle) (values map[pairKey]Value, failed map[string]error) {
	values, failed = make(map[pairKey]Value, len(subscriptions)), map[string]error{}
	for _, subscription := range subscriptions {
		candles := histories[historyKey{subscription.InstrumentID, subscription.Target.Interval}]
		value, err := Calculate(tracker.registry, subscription.Target, candles[max(0, len(candles)-subscription.depth):], subscription.points())
		if err != nil {
			failed[subscription.Target.Key()] = err
			continue
		}
		values[pairKey{subscription.InstrumentID, subscription.Target.Key()}] = value
	}
	return values, failed
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
