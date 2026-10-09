package pivot_test

import (
	"slices"
	"testing"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/pivot"
)

func TestPivotsCarryTheLatestAndPreviousConfirmedPivots(t *testing.T) {
	// Highs pivot at 1 and, of the equal highs at 4 and 5, at the later one,
	// as ta.pivothigh picks it; lows pivot at 3 and 7.
	highs := []float64{1, 5, 2, 2, 4, 4, 1, 1, 1}
	lows := []float64{5, 4, 4, 1, 4, 4, 4, 2, 3}
	result, err := pivot.New().Calculate(indicator.Parameters{"left": 1, "right": 1}, indicator.Inputs{"high": highs, "low": lows})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]indicator.Series{
		"high":          {Offset: 2, Values: []float64{5, 5, 5, 5, 4, 4, 4}},
		"previous_high": {Offset: 6, Values: []float64{5, 5, 5}},
		"low":           {Offset: 4, Values: []float64{1, 1, 1, 1, 2}},
		"previous_low":  {Offset: 8, Values: []float64{1}},
	}
	for name, series := range want {
		got := result.Outputs[name]
		if got.Offset != series.Offset || !slices.Equal(got.Values, series.Values) {
			t.Errorf("%s = %+v, want %+v", name, got, series)
		}
	}
}

func TestPivotsCountOnlyWithinTheirWindowWhateverHistoryCallersPass(t *testing.T) {
	// The only pivot high is at 1, with its left candle at 0: the 500
	// candles ending at 499 hold it, those ending at 500 do not.
	for _, test := range []struct {
		candles int
		want    indicator.Series
	}{
		{candles: 500, want: indicator.Series{Offset: 2, Values: slices.Repeat([]float64{5}, 498)}},
		{candles: 501, want: indicator.Series{Offset: 501, Values: []float64{}}},
	} {
		highs := slices.Repeat([]float64{1}, test.candles)
		highs[1] = 5
		lows := slices.Repeat([]float64{1}, test.candles)
		result, err := pivot.New().Calculate(indicator.Parameters{"left": 1, "right": 1}, indicator.Inputs{"high": highs, "low": lows})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Outputs["high"]; got.Offset != test.want.Offset || !slices.Equal(got.Values, test.want.Values) {
			t.Errorf("%d candles: high offset %d with %d values, want offset %d with %d", test.candles, got.Offset, len(got.Values), test.want.Offset, len(test.want.Values))
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
	for name, candles := range want {
		series := result.Outputs[name]
		if series.Offset != offset {
			t.Fatalf("%s offset = %d, want %d", name, series.Offset, offset)
		}
		var marked []int
		for index, value := range series.Values {
			if value != 0 {
				marked = append(marked, offset+index)
			}
		}
		if !slices.Equal(marked, candles) {
			t.Errorf("%s marks candles %v, want %v", name, marked, candles)
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
