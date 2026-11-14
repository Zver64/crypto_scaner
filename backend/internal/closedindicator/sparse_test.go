package closedindicator_test

import (
	"testing"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/pivot"
	"crypto-scanner/internal/market"
)

func TestEarlierValuesBehindAnEmptyOneStayReadable(t *testing.T) {
	registry, err := indicator.NewRegistry(pivot.New())
	if err != nil {
		t.Fatal(err)
	}
	// The only pivot high, at the third candle, is confirmed at the fourth,
	// two candles before the latest.
	candles := testHistory(6)
	for index, high := range []float64{1, 2, 9, 3, 2, 1} {
		candles[index].High = high
	}
	target := closedindicator.Target{Interval: market.IntervalHour, Selection: indicator.Selection{Type: pivot.Type, Parameters: indicator.Parameters{"left": 1, "right": 1}}}
	value, err := closedindicator.Calculate(registry, target, candles, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range value.Outputs {
		if output.Name != "high" {
			continue
		}
		for shift, want := range []float64{0, 0, 9, 0} {
			got, known := output.At(shift)
			if !known || indicator.Empty(got) != (want == 0) || want != 0 && got != want {
				t.Fatalf("high %d candles back = %v (%v), want %v, 0 for empty", shift, got, known, want)
			}
		}
		if _, shown := output.Shown(); shown {
			t.Fatal("an empty value is shown")
		}
		return
	}
	t.Fatal("no high output")
}
