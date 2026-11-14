package talib_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/indicator/pivot"
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
			if !errors.Is(err, indicator.ErrInvalidRequest) {
				t.Fatalf("Calculate() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestAdapterNormalizesEqualSettingsEqually(t *testing.T) {
	registry := newRegistry(t)
	want := indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14, "source": 3}}
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

// rsiSource is the RSI of period as the source of another indicator.
func rsiSource(period int) map[string]any {
	return indicator.Source{Selection: indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": period}}, Output: "rsi"}.Value()
}

func TestIndicatorOfAnIndicatorReadsItsValues(t *testing.T) {
	registry := newRegistry(t)
	closes := make([]float64, 40)
	for index := range closes {
		closes[index] = 100 + 10*math.Sin(float64(index))
	}
	inputs := indicator.Inputs{"close": closes}
	rsi, err := registry.Calculate(indicator.Request{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 5}, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	// The SMA of the bare RSI values, read as if they were closes.
	want, err := registry.Calculate(indicator.Request{Type: "sma", Parameters: indicator.Parameters{"period": 3}, Inputs: indicator.Inputs{"close": rsi.Outputs["rsi"].Values}})
	if err != nil {
		t.Fatal(err)
	}
	nested := indicator.Parameters{"period": 3, "source": rsiSource(5)}
	got, err := registry.Calculate(indicator.Request{Type: "sma", Parameters: nested, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	// The RSI warms up over 5 candles and the SMA over 2 of its values.
	if series := got.Outputs["sma"]; series.Offset != 7 || !reflect.DeepEqual(series.Values, want.Outputs["sma"].Values) {
		t.Fatalf("sma of rsi = %v from %d, want %v from 7", series.Values, series.Offset, want.Outputs["sma"].Values)
	}
	if lookback, err := registry.Lookback("sma", nested); err != nil || lookback != 7 {
		t.Fatalf("Lookback() = %d, %v; want 7, nil", lookback, err)
	}
	if fields, err := registry.Fields(indicator.Selection{Type: "sma", Parameters: nested}); err != nil || !reflect.DeepEqual(fields, []string{"close"}) {
		t.Fatalf("Fields() = %v, %v; want [close], nil", fields, err)
	}
	// Before the source has values there is nothing, not a failure.
	short, err := registry.Calculate(indicator.Request{Type: "sma", Parameters: nested, Inputs: indicator.Inputs{"close": closes[:4]}})
	if err != nil || len(short.Outputs["sma"].Values) != 0 {
		t.Fatalf("sma of rsi over a short history = %v, %v; want no values", short.Outputs["sma"], err)
	}
}

func TestNestedSourcesAreNormalizedAndChecked(t *testing.T) {
	registry, err := indicator.NewRegistry(append(indicatortalib.New(), pivot.New(), candle.New())...)
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(source any) (indicator.Selection, error) {
		return registry.Normalize(indicator.Selection{Type: "sma", Parameters: indicator.Parameters{"source": source}})
	}
	// Defaults are filled in at both levels, so equal settings compare equal.
	got, err := normalize(map[string]any{"type": "rsi", "output": "rsi"})
	want := indicator.Parameters{"period": 30, "source": map[string]any{"type": "rsi", "output": "rsi", "parameters": map[string]any{"period": 14, "source": 3}}}
	if err != nil || !reflect.DeepEqual(got.Parameters, want) {
		t.Fatalf("Normalize() = %#v, %v; want %#v", got.Parameters, err, want)
	}
	rejected := map[string]any{
		"two levels":     indicator.Source{Selection: indicator.Selection{Type: "ema", Parameters: indicator.Parameters{"source": rsiSource(14)}}, Output: "ema"}.Value(),
		"sparse output":  indicator.Source{Selection: indicator.Selection{Type: pivot.Type}, Output: "high"}.Value(),
		"unknown output": indicator.Source{Selection: indicator.Selection{Type: "macd"}, Output: "rsi"}.Value(),
		"internal type":  indicator.Source{Selection: indicator.Selection{Type: candle.Type}, Output: "close"}.Value(),
		"unknown type":   indicator.Source{Selection: indicator.Selection{Type: "nothing"}, Output: "nothing"}.Value(),
		"bad parameters": rsiSource(1),
		"unknown field":  7,
		"not a source":   "close",
	}
	for name, source := range rejected {
		if _, err := normalize(source); !errors.Is(err, indicator.ErrInvalidRequest) {
			t.Errorf("%s: Normalize() error = %v, want ErrInvalidRequest", name, err)
		}
	}
}
