package talib_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

func newRegistry(t *testing.T) *indicator.Registry {
	t.Helper()
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestAdapterValidatesParametersAndInputs(t *testing.T) {
	registry := newRegistry(t)
	closes := []float64{1, 2, 3, 4, 5}
	tests := []struct {
		name          string
		indicatorType indicator.Type
		parameters    indicator.Parameters
		inputs        indicator.Inputs
	}{
		{name: "string period", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": "14"}, inputs: indicator.Inputs{"close": closes}},
		{name: "fractional period", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": 2.5}, inputs: indicator.Inputs{"close": closes}},
		{name: "period too small", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": 1}, inputs: indicator.Inputs{"close": closes}},
		{name: "period beyond history depth", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": 2001}, inputs: indicator.Inputs{"close": closes}},
		{name: "unknown parameter", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"length": 14}, inputs: indicator.Inputs{"close": closes}},
		{name: "unknown moving average type", indicatorType: "ma", parameters: indicator.Parameters{"ma_type": 9}, inputs: indicator.Inputs{"close": closes}},
		{name: "infinite real parameter", indicatorType: "bbands", parameters: indicator.Parameters{"deviations_up": math.Inf(1)}, inputs: indicator.Inputs{"close": closes}},
		{name: "missing input", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": 2}},
		{name: "NaN input", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": 2}, inputs: indicator.Inputs{"close": {1, math.NaN(), 3}}},
		{name: "infinite input", indicatorType: indicatortalib.RSIType, parameters: indicator.Parameters{"period": 2}, inputs: indicator.Inputs{"close": {1, math.Inf(1), 3}}},
		{name: "inputs of different lengths", indicatorType: "atr", parameters: indicator.Parameters{"period": 2}, inputs: indicator.Inputs{"high": {2, 3, 4}, "low": {1, 2}, "close": {1, 2, 3}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := registry.Calculate(indicator.Request{Type: test.indicatorType, Parameters: test.parameters, Inputs: test.inputs})
			if !errors.Is(err, indicatortalib.ErrInvalidRequest) {
				t.Fatalf("Calculate() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestAdapterNormalizesEqualSettingsEqually(t *testing.T) {
	registry := newRegistry(t)
	want := indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}
	for _, parameters := range []indicator.Parameters{nil, {"period": 14}, {"period": float64(14)}, {"period": int64(14)}} {
		got, err := registry.Normalize(indicator.Selection{Type: indicatortalib.RSIType, Parameters: parameters})
		if err != nil {
			t.Fatalf("Normalize(%v) error = %v", parameters, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Normalize(%v) = %#v, want %#v", parameters, got, want)
		}
	}
}

func TestAdapterReturnsNonNilEmptySeriesBeforeWarmup(t *testing.T) {
	registry := newRegistry(t)
	result, err := registry.Calculate(indicator.Request{
		Type: "macd", Parameters: indicator.Parameters{"fast_period": 3, "slow_period": 5, "signal_period": 2},
		Inputs: indicator.Inputs{"close": {1, 2, 3, 4, 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"macd", "macdsignal", "macdhist"} {
		series := result.Outputs[name]
		if series.Offset != 5 || series.Values == nil || len(series.Values) != 0 {
			t.Fatalf("%s = %#v, want non-nil empty values at offset 5", name, series)
		}
	}
}
