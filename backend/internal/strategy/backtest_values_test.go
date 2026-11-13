package strategy

import (
	"maps"
	"math"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// Every value a replay reads at a candle is the value of its indicator over
// the window ending there, calculated from scratch through the indicator
// implementation, whatever the replay reuses between candles: single and
// multi-output, recursive and pivot-based indicators, Prev shifts, a gap,
// a coarser interval, and windows both before and after the history fills
// market.SyncDepth.
func TestReplayedValuesMatchAFreshCalculationOfEachWindow(t *testing.T) {
	registry := testRegistry(t)
	const count = market.SyncDepth + 300
	var hourly []market.Candle
	for index := range count {
		open := backtestHour(index)
		if index >= 1500 {
			// One missing hour, so windows span two contiguous runs.
			open = backtestHour(index + 1)
		}
		price := 100 + 10*math.Sin(float64(index)/23) + 3*math.Sin(float64(index)/5)
		candle := testCandle(1, market.IntervalHour, open, price)
		candle.Open, candle.High, candle.Low = price-math.Cos(float64(index)), price+1+math.Sin(float64(index)/3), price-1-math.Cos(float64(index)/4)
		candle.Volume, candle.TradeCount = 10+float64(index%17), int64(100+index%29)
		hourly = append(hourly, candle)
	}
	var daily []market.Candle
	for index := range count/24 + 2 {
		daily = append(daily, testCandle(1, market.IntervalDay, backtestStart.AddDate(0, 0, index), 100+float64(index*index%17)))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalDay] = map[int64][]market.Candle{1: daily}
	compiled, err := Compile(`ema("1h", 100) > Prev(ema("1h", 100)) && macd_macdsignal("1h") > Prev(rsi("1h"), 50) && bbands_upperband("1h", 20) > close("1h") && `+
		`pivot_low("1h") < Prev(close("1h"), pivot_low_bars("1h")) && divergence_bull("1h") >= 0 && Prev(close("1h"), 3) > ema("1d", 5)`, registry, EntryRule)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{Strategy: Strategy{ID: testID(1), Enabled: true}, Compiled: compiled, Interval: market.IntervalHour, reads: compiled.Reads()}
	instruments := []Instrument{{ID: 1, Symbol: "BTCUSDT"}}
	replay, err := newReplay(registry, entry, instruments[0], instruments)
	if err != nil {
		t.Fatal(err)
	}
	replay.histories = store.candles
	// compared counts the compared values of each subscription.
	compared, cold := make([]int, len(replay.current.reads.subscriptions)), 0
	for _, candle := range hourly {
		now := market.IntervalHour.NextOpenTime(candle.OpenTime)
		current, _, err := replay.at(now)
		if err != nil {
			t.Fatal(err)
		}
		for index, subscription := range current.reads.subscriptions {
			if replay.cut[index] {
				cold++
				continue
			}
			history := store.candles[subscription.Target.Interval][subscription.InstrumentID]
			open := subscription.Target.Interval.LastClosedOpenTime(now)
			end, found := slices.BinarySearchFunc(history, open, func(candle market.Candle, open time.Time) int { return candle.OpenTime.Compare(open) })
			if found {
				end++
			}
			depth, err := closedindicator.Depth(registry, subscription.Target, subscription.Points)
			if err != nil {
				t.Fatal(err)
			}
			want := freshValue(t, registry, subscription.Target, history[max(0, end-depth):end], max(1, subscription.Points))
			if got := current.values[index]; !sameValue(got, want) {
				t.Fatalf("%s at %s = %+v, want %+v", subscription.Target.Key(), now, got, want)
			}
			compared[index]++
		}
	}
	if slices.Contains(compared, 0) || cold == 0 {
		t.Fatalf("compared %v values of each subscription and %d cold ones", compared, cold)
	}
}

// freshValue is the value of target, with points−1 earlier values, over
// window, calculated directly through the implementation over the last
// contiguous run of window.
func freshValue(t *testing.T, registry *indicator.Registry, target closedindicator.Target, window []market.Candle, points int) closedindicator.Value {
	t.Helper()
	value := closedindicator.Value{Target: target}
	if len(window) == 0 {
		return value
	}
	value.OpenTime = window[len(window)-1].OpenTime.UTC()
	start := len(window) - 1
	for start > 0 && target.Interval.NextOpenTime(window[start-1].OpenTime).Equal(window[start].OpenTime) {
		start--
	}
	run := window[start:]
	fields, err := registry.Fields(target.Selection)
	if err != nil {
		t.Fatal(err)
	}
	inputs := indicator.Inputs{}
	for _, field := range fields {
		values := make([]float64, len(run))
		for index, candle := range run {
			values[index] = map[string]float64{
				"open": candle.Open, "high": candle.High, "low": candle.Low, "close": candle.Close,
				"volume": candle.Volume, "quote_volume": candle.QuoteAssetVolume, "trades": float64(candle.TradeCount),
			}[field]
		}
		inputs[field] = values
	}
	result, err := registry.Calculate(indicator.Request{Type: target.Selection.Type, Parameters: target.Selection.Parameters, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(result.Outputs)) {
		series := result.Outputs[name]
		last := len(series.Values) - 1
		if last < 0 || series.Offset+last != len(run)-1 {
			continue
		}
		output := closedindicator.Output{Name: name, Value: series.Values[last]}
		for index := last - 1; index >= 0 && len(output.Earlier) < points-1; index-- {
			output.Earlier = append(output.Earlier, series.Values[index])
		}
		value.Outputs = append(value.Outputs, output)
	}
	return value
}

func sameValue(left, right closedindicator.Value) bool {
	return left.Target.Equal(right.Target) && left.OpenTime.Equal(right.OpenTime) && slices.EqualFunc(left.Outputs, right.Outputs, func(left, right closedindicator.Output) bool {
		return left.Name == right.Name && left.Value == right.Value && slices.Equal(left.Earlier, right.Earlier)
	})
}
