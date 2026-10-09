// Package pivot provides the swing indicators TA-Lib lacks: confirmed pivot
// highs and lows, and the RSI divergence between them, both as TradingView
// calculates them (ta.pivothigh, ta.pivotlow, and its RSI Divergence
// Indicator).
package pivot

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
)

const (
	// Type is the registry identifier of the pivot highs and lows.
	Type indicator.Type = "pivot"
	// group lists both modules together in the indicator picker.
	group = "Pivots"
	// maxSide bounds the candles a pivot is compared with on either side,
	// which every candle of a backtest compares again.
	maxSide = 20
	// window is how many candles, ending at a candle, its pivots are found
	// in.
	window = 500
	// reach is the lookback for which chart.Window loads that window.
	reach = window/chart.SettleFactor - 1
)

// ErrInvalidRequest indicates parameters or inputs the modules reject.
var ErrInvalidRequest = errors.New("invalid pivot request")

// sides are the TradingView pivot lookbacks: the candles before and after a
// pivot that it must stand out from.
var sides = []param{
	{key: "left", title: "Pivot Lookback Left", description: "Candles before a pivot", minimum: 1, maximum: maxSide, defaultValue: 5},
	{key: "right", title: "Pivot Lookback Right", description: "Candles after a pivot, which confirm it", minimum: 1, maximum: maxSide, defaultValue: 5},
}

// New returns the pivot high and low module.
func New() indicator.Implementation { return pivots{} }

// pivots reports, at every candle, the latest and the previous pivot high and
// low confirmed by then within the window, and how many candles back each
// lies.
type pivots struct{}

func (pivots) Describe() indicator.Descriptor {
	return indicator.Descriptor{
		Type: Type, Title: "Pivot High/Low", Group: group, Overlay: true, Unstable: true,
		Inputs:     []string{"high", "low"},
		Parameters: describe(sides),
		Outputs: []indicator.OutputDescriptor{
			{Name: "high", Style: indicator.OutputLine},
			{Name: "previous_high", Style: indicator.OutputDashedLine},
			{Name: "low", Style: indicator.OutputLine},
			{Name: "previous_low", Style: indicator.OutputDashedLine},
			{Name: "high_bars", Style: indicator.OutputHidden, Count: true},
			{Name: "previous_high_bars", Style: indicator.OutputHidden, Count: true},
			{Name: "low_bars", Style: indicator.OutputHidden, Count: true},
			{Name: "previous_low_bars", Style: indicator.OutputHidden, Count: true},
		},
	}
}

func (pivots) Normalize(parameters indicator.Parameters) (indicator.Parameters, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return nil, err
	}
	return canonical(sides, values), nil
}

func (m pivots) Fields(parameters indicator.Parameters) ([]string, error) {
	if _, err := m.Normalize(parameters); err != nil {
		return nil, err
	}
	return []string{"high", "low"}, nil
}

// Lookback is the reach that makes chart.Window load the window, whose every
// candle a value may depend on, rather than the first value, which comes with
// the first confirmed pivot.
func (pivots) Lookback(parameters indicator.Parameters) (int, error) {
	if _, err := parse(sides, parameters); err != nil {
		return 0, err
	}
	return reach, nil
}

func (pivots) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, []string{"high", "low"})
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	left, right := values[0], values[1]
	highs := confirmed(series[0], left, right, isHigh)
	lows := confirmed(series[1], left, right, isLow)
	high, previousHigh := levels(series[0], highs, left, right)
	low, previousLow := levels(series[1], lows, left, right)
	outputs := indicator.Outputs{
		"high": high.values, "previous_high": previousHigh.values, "low": low.values, "previous_low": previousLow.values,
		"high_bars": high.bars, "previous_high_bars": previousHigh.bars, "low_bars": low.bars, "previous_low_bars": previousLow.bars,
	}
	return indicator.Result{Outputs: outputs}, nil
}

// level is one pivot series: its value at every candle and how many candles
// back it lies.
type level struct {
	values, bars indicator.Series
}

// levels reports at every candle the latest and the previous pivot confirmed
// by then, right candles after it, that lies with its left candles within the
// window ending there, so a value does not depend on how much more history
// the caller passes. A series holds only consecutive values, so each keeps
// its last unbroken run: a pivot leaving the window before the next is
// confirmed drops the values before.
func levels(values []float64, pivots []int, left, right int) (latest, previous level) {
	for _, series := range []*level{&latest, &previous} {
		series.values.Values, series.bars.Values = []float64{}, []float64{}
	}
	next := 0
	for at := range values {
		for next < len(pivots) && pivots[next]+right <= at {
			next++
		}
		// keep extends series with the pivot back pivots before the latest.
		keep := func(series *level, back int) {
			if next > back && pivots[next-1-back]-left > at-window {
				pivot := pivots[next-1-back]
				series.values.Values = append(series.values.Values, values[pivot])
				series.bars.Values = append(series.bars.Values, float64(at-pivot))
				return
			}
			series.values.Offset, series.values.Values = at+1, series.values.Values[:0]
			series.bars.Offset, series.bars.Values = at+1, series.bars.Values[:0]
		}
		keep(&latest, 0)
		keep(&previous, 1)
	}
	return latest, previous
}

// confirmed lists the pivots of values in order, positions with left valid
// candles before them and right after them.
func confirmed(values []float64, left, right int, pivot func(values []float64, at, left, right int) bool) []int {
	var result []int
	for at := left; at+right < len(values); at++ {
		if pivot(values, at, left, right) {
			result = append(result, at)
		}
	}
	return result
}

// isHigh reports a pivot high at at the way ta.pivothigh finds it: no candle
// before it is higher and every candle after it is lower, so of equal highs
// the latest becomes the pivot.
func isHigh(values []float64, at, left, right int) bool {
	for index := at - left; index < at; index++ {
		if values[index] > values[at] {
			return false
		}
	}
	for index := at + 1; index <= at+right; index++ {
		if values[index] >= values[at] {
			return false
		}
	}
	return true
}

// isLow mirrors isHigh, as ta.pivotlow does.
func isLow(values []float64, at, left, right int) bool {
	for index := at - left; index < at; index++ {
		if values[index] < values[at] {
			return false
		}
	}
	for index := at + 1; index <= at+right; index++ {
		if values[index] <= values[at] {
			return false
		}
	}
	return true
}

// param is one integer parameter of these modules.
type param struct {
	key          string
	title        string
	description  string
	minimum      int
	maximum      int
	defaultValue int
}

func describe(params []param) []indicator.ParameterDescriptor {
	result := make([]indicator.ParameterDescriptor, len(params))
	for index, p := range params {
		result[index] = indicator.ParameterDescriptor{
			Key: p.key, Title: p.title, Description: p.description, Kind: indicator.ParameterInteger,
			Default: float64(p.defaultValue), Minimum: float64(p.minimum), Maximum: float64(p.maximum),
		}
	}
	return result
}

func canonical(params []param, values []int) indicator.Parameters {
	result := make(indicator.Parameters, len(params))
	for index, p := range params {
		result[p.key] = values[index]
	}
	return result
}

// parse validates parameters and returns them in params order with defaults
// for omitted keys.
func parse(params []param, parameters indicator.Parameters) ([]int, error) {
	for key := range parameters {
		if !slices.ContainsFunc(params, func(p param) bool { return p.key == key }) {
			return nil, fmt.Errorf("%w: no parameter %q", ErrInvalidRequest, key)
		}
	}
	values := make([]int, len(params))
	for index, p := range params {
		raw, exists := parameters[p.key]
		if !exists {
			values[index] = p.defaultValue
			continue
		}
		value, err := indicator.ParseNumber(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %w", ErrInvalidRequest, p.key, err)
		}
		if math.Trunc(value) != value || value < float64(p.minimum) || value > float64(p.maximum) {
			return nil, fmt.Errorf("%w: %s must be an integer between %d and %d", ErrInvalidRequest, p.key, p.minimum, p.maximum)
		}
		values[index] = int(value)
	}
	return values, nil
}
