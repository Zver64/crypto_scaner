package talib

import (
	"errors"
	"math"
	"testing"

	"crypto-scanner/internal/indicator"
)

func TestSingleInputPeriodAdaptsCalculationAndWarmup(t *testing.T) {
	var receivedPeriod int
	var receivedValues []float64
	implementation := newSingleInputPeriod(singleInputPeriodSpec{
		indicatorType: "fake",
		inputName:     "price",
		outputName:    "value",
		minimumPeriod: 2,
		maximumPeriod: 20,
		offset:        func(period int) int { return period },
		lookback:      func(period int) int { return period * 3 },
		calculate: func(values []float64, period int) []float64 {
			receivedPeriod = period
			receivedValues = values
			output := make([]float64, len(values))
			for index := range output {
				output[index] = float64(index + period)
			}
			return output
		},
	})

	lookback, err := implementation.Lookback(indicator.Parameters{"period": float64(3)})
	if err != nil || lookback != 9 {
		t.Fatalf("Lookback() = %d, %v; want 9, nil", lookback, err)
	}
	inputs := indicator.Inputs{"price": {10, 11, 12, 13, 14}}
	result, err := implementation.Calculate(indicator.Parameters{"period": 3}, inputs)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	series := result.Outputs["value"]
	if receivedPeriod != 3 || len(receivedValues) != 5 {
		t.Fatalf("calculator received period %d and %d values", receivedPeriod, len(receivedValues))
	}
	if series.Offset != 3 || len(series.Values) != 2 || series.Values[0] != 6 || series.Values[1] != 7 {
		t.Fatalf("series = %#v, want offset 3 and values [6 7]", series)
	}
}

func TestSingleInputPeriodReturnsNonNilEmptySeriesBeforeWarmup(t *testing.T) {
	called := false
	implementation := newSingleInputPeriod(singleInputPeriodSpec{
		indicatorType: "fake",
		inputName:     "price",
		outputName:    "value",
		minimumPeriod: 2,
		maximumPeriod: 20,
		offset:        func(period int) int { return period },
		lookback:      func(period int) int { return period },
		calculate: func([]float64, int) []float64 {
			called = true
			return nil
		},
	})
	result, err := implementation.Calculate(indicator.Parameters{"period": 5}, indicator.Inputs{"price": {1, 2, 3, 4, 5}})
	if err != nil {
		t.Fatal(err)
	}
	series := result.Outputs["value"]
	if called || series.Offset != 5 || series.Values == nil || len(series.Values) != 0 {
		t.Fatalf("called=%v series=%#v, want no call and non-nil empty values at offset 5", called, series)
	}
}

func TestSingleInputPeriodValidatesSharedContract(t *testing.T) {
	implementation := newSingleInputPeriod(singleInputPeriodSpec{
		indicatorType: "fake",
		inputName:     "price",
		outputName:    "value",
		minimumPeriod: 2,
		maximumPeriod: 20,
		offset:        func(period int) int { return period },
		lookback:      func(period int) int { return period },
		calculate:     func(values []float64, _ int) []float64 { return make([]float64, len(values)) },
	})
	tests := []struct {
		name       string
		parameters indicator.Parameters
		inputs     indicator.Inputs
	}{
		{name: "missing period", inputs: indicator.Inputs{"price": {1, 2}}},
		{name: "string period", parameters: indicator.Parameters{"period": "14"}, inputs: indicator.Inputs{"price": {1, 2}}},
		{name: "fractional period", parameters: indicator.Parameters{"period": 2.5}, inputs: indicator.Inputs{"price": {1, 2, 3}}},
		{name: "period too small", parameters: indicator.Parameters{"period": 1}, inputs: indicator.Inputs{"price": {1, 2}}},
		{name: "period too large", parameters: indicator.Parameters{"period": 21}, inputs: indicator.Inputs{"price": {1, 2}}},
		{name: "missing input", parameters: indicator.Parameters{"period": 2}},
		{name: "NaN input", parameters: indicator.Parameters{"period": 2}, inputs: indicator.Inputs{"price": {1, math.NaN()}}},
		{name: "infinite input", parameters: indicator.Parameters{"period": 2}, inputs: indicator.Inputs{"price": {1, math.Inf(1)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := implementation.Calculate(test.parameters, test.inputs)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Calculate() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}
