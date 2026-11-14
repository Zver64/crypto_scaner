package pivot_test

import (
	"maps"
	"slices"
	"testing"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/pivot"
)

func TestPivotsHoldTheirPriceOnTheConfirmingCandle(t *testing.T) {
	// Highs pivot at 1 and, of the equal highs at 4 and 5, at the later one,
	// as ta.pivothigh picks it; lows pivot at 3 and 7. Each is confirmed one
	// candle later.
	highs := []float64{1, 5, 2, 2, 4, 4, 1, 1, 1}
	lows := []float64{5, 4, 4, 1, 4, 4, 4, 2, 3}
	result, err := pivot.New().Calculate(indicator.Parameters{"left": 1, "right": 1}, indicator.Inputs{"high": highs, "low": lows})
	if err != nil {
		t.Fatal(err)
	}
	// The first candle that can confirm a pivot is the third.
	const offset = 2
	for name, want := range map[string]map[int]float64{"high": {2: 5, 6: 4}, "low": {4: 1, 8: 2}} {
		series := result.Outputs[name]
		if series.Offset != offset || len(series.Values) != len(highs)-offset {
			t.Fatalf("%s covers %d candles from %d, want %d from %d", name, len(series.Values), series.Offset, len(highs)-offset, offset)
		}
		got := map[int]float64{}
		for index, value := range series.Values {
			if !indicator.Empty(value) {
				got[offset+index] = value
			}
		}
		if !maps.Equal(got, want) {
			t.Errorf("%s = %v at candles, want %v", name, got, want)
		}
	}
}

// fakeRSI returns the given values as the RSI, without warm-up.
type fakeRSI struct {
	indicator.Implementation
	values []float64
}

func (fakeRSI) Lookback(indicator.Parameters) (int, error) { return 0, nil }

func (rsi fakeRSI) Calculate(indicator.Parameters, indicator.Inputs) (indicator.Result, error) {
	return indicator.Result{Outputs: indicator.Outputs{"rsi": {Values: rsi.values}}}, nil
}

func TestDivergenceComparesThePriceAtConsecutiveRSIPivots(t *testing.T) {
	// Of the RSI pivots, the lows at 1, 4, and 7 and the highs at 10 and 13
	// diverge: the price makes a lower low at 4 while the RSI rises
	// (bullish), a higher low at 7 while the RSI falls (hidden bullish), and a
	// higher high at 13 while the RSI falls (bearish). Each is confirmed one
	// candle later.
	rsi := []float64{50, 30, 50, 50, 40, 50, 50, 35, 50, 50, 70, 50, 50, 60, 50, 50}
	lows := []float64{9, 5, 9, 9, 4, 9, 9, 6, 9, 9, 9, 9, 9, 9, 9, 9}
	highs := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 20, 10, 10, 21, 10, 10}
	closes := make([]float64, len(rsi))
	parameters := indicator.Parameters{"left": 1, "right": 1, "range_lower": 0, "range_upper": 2}
	result, err := pivot.NewDivergence(fakeRSI{values: rsi}).Calculate(parameters, indicator.Inputs{"close": closes, "high": highs, "low": lows})
	if err != nil {
		t.Fatal(err)
	}
	// The first value has a previous pivot up to range_upper candles back.
	const offset = 5
	want := map[string][]int{"bull": {5}, "hidden_bull": {8}, "bear": {14}, "hidden_bear": nil}
	// On those candles, the earlier pivot of their side lies the distance
	// between the pivots plus right candles back.
	bars := map[string]map[int]float64{"previous_low_bars": {5: 4, 8: 4}, "previous_high_bars": {14: 4}}
	for name, candles := range want {
		bars[name] = map[int]float64{}
		for _, candle := range candles {
			bars[name][candle] = 1
		}
	}
	for name, values := range bars {
		series := result.Outputs[name]
		if series.Offset != offset {
			t.Fatalf("%s offset = %d, want %d", name, series.Offset, offset)
		}
		marked := map[int]float64{}
		for index, value := range series.Values {
			if value != 0 {
				marked[offset+index] = value
			}
		}
		if !maps.Equal(marked, values) {
			t.Errorf("%s = %v at candles, want %v", name, marked, values)
		}
	}

	// Two candles lie between the pivots, past a range of at most one.
	parameters["range_upper"] = 1
	result, err = pivot.NewDivergence(fakeRSI{values: rsi}).Calculate(parameters, indicator.Inputs{"close": closes, "high": highs, "low": lows})
	if err != nil {
		t.Fatal(err)
	}
	for name, series := range result.Outputs {
		if slices.Contains(series.Values, 1) {
			t.Errorf("%s marks a divergence beyond the range", name)
		}
	}
}
