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

// With a source, pivot highs and lows are both found in that series, which
// the registry supplies: a candle field or the values of another indicator.
func TestPivotsOfASourceAreFoundInThatSeries(t *testing.T) {
	registry, err := indicator.NewRegistry(pivot.New())
	if err != nil {
		t.Fatal(err)
	}
	closes := []float64{3, 5, 3, 2, 1, 4, 4, 3, 3}
	parameters := indicator.Parameters{"left": 1, "right": 1, "source": slices.Index(indicator.CandleFields, "close")}
	if fields, err := registry.Fields(indicator.Selection{Type: pivot.Type, Parameters: parameters}); err != nil || !slices.Equal(fields, []string{"close"}) {
		t.Fatalf("Fields() = %v, %v; want [close], nil", fields, err)
	}
	result, err := registry.Calculate(indicator.Request{Type: pivot.Type, Parameters: parameters, Inputs: indicator.Inputs{"close": closes}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]map[int]float64{"high": {2: 5, 7: 4}, "low": {5: 1}} {
		series := result.Outputs[name]
		got := map[int]float64{}
		for index, value := range series.Values {
			if !indicator.Empty(value) {
				got[series.Offset+index] = value
			}
		}
		if !maps.Equal(got, want) {
			t.Errorf("%s = %v at candles, want %v", name, got, want)
		}
	}
}
