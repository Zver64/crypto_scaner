package talib_test

import (
	"math"
	"testing"

	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

func TestRSIBindingProducesKnownWilderSequenceWithoutWarmupZeros(t *testing.T) {
	implementation := indicatortalib.NewRSI()
	closeValues := []float64{54.8, 56.8, 57.85, 59.85, 60.57, 61.1, 62.17, 60.6, 62.35, 62.15, 62.35, 61.45, 62.8, 61.37, 62.5, 62.57, 60.8, 59.37, 60.35, 62.35}
	result, err := implementation.Calculate(indicator.Parameters{"period": 14}, indicator.Inputs{"close": closeValues})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	series := result.Outputs["rsi"]
	if series.Offset != 14 {
		t.Fatalf("Offset = %d, want 14", series.Offset)
	}
	want := []float64{74.21383647798743, 74.33551617873653, 65.87128621880291, 59.933703637340976, 62.43287589053954, 66.96204697604155}
	if len(series.Values) != len(want) {
		t.Fatalf("len(Values) = %d, want %d", len(series.Values), len(want))
	}
	for index := range want {
		if math.Abs(series.Values[index]-want[index]) > 1e-10 {
			t.Errorf("Values[%d] = %.12f, want %.12f", index, series.Values[index], want[index])
		}
	}
	lookback, err := implementation.Lookback(indicator.Parameters{"period": float64(14)})
	if err != nil || lookback != 140 {
		t.Fatalf("Lookback() = %d, %v; want 140, nil", lookback, err)
	}
}
