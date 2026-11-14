// Package pivot provides the swing indicators TA-Lib lacks: confirmed pivot
// highs and lows, and the RSI divergence between them, both as TradingView
// calculates them (ta.pivothigh, ta.pivotlow, and its RSI Divergence
// Indicator).
package pivot

import (
	"fmt"
	"math"
	"slices"

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
)

// sides are the TradingView pivot lookbacks: the candles before and after a
// pivot that it must stand out from.
var sides = []param{
	{key: "left", title: "Pivot Lookback Left", description: "Candles before a pivot", minimum: 1, maximum: maxSide, defaultValue: 5},
	{key: "right", title: "Pivot Lookback Right", description: "Candles after a pivot, which confirm it", minimum: 1, maximum: maxSide, defaultValue: 5},
}

// New returns the pivot high and low module.
func New() indicator.Implementation { return pivots{} }

// pivots reports the price of a pivot high and of a pivot low on the candle
// that confirms it, right candles after the pivot, and is empty on every
// other candle.
type pivots struct{}

func (pivots) Describe() indicator.Descriptor {
	return indicator.Descriptor{
		Type: Type, Title: "Pivot High/Low", Group: group, Overlay: true,
		Inputs:     []string{"high", "low"},
		Parameters: describe(sides),
		Outputs: []indicator.OutputDescriptor{
			{Name: "high", Style: indicator.OutputMarkerAbove, Sparse: true, Shift: "right"},
			{Name: "low", Style: indicator.OutputMarkerBelow, Sparse: true, Shift: "right"},
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

// Lookback is the candles a value depends on besides its own: the left
// candles before the pivot and the right after it.
func (pivots) Lookback(parameters indicator.Parameters) (int, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return 0, err
	}
	return values[0] + values[1], nil
}

func (pivots) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, []string{"high", "low"})
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", indicator.ErrInvalidRequest, err)
	}
	left, right := values[0], values[1]
	return indicator.Result{Outputs: indicator.Outputs{
		"high": confirmations(series[0], left, right, isHigh),
		"low":  confirmations(series[1], left, right, isLow),
	}}, nil
}

// confirmations holds, from the first candle that can confirm a pivot, the
// value of each pivot on the candle right after it and nothing elsewhere.
func confirmations(values []float64, left, right int, pivot func(values []float64, at, left, right int) bool) indicator.Series {
	series := indicator.Series{Offset: left + right, Values: []float64{}}
	for range max(0, len(values)-series.Offset) {
		series.Values = append(series.Values, math.NaN())
	}
	for _, at := range confirmed(values, left, right, pivot) {
		series.Values[at-left] = values[at]
	}
	return series
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
			return nil, fmt.Errorf("%w: no parameter %q", indicator.ErrInvalidRequest, key)
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
			return nil, fmt.Errorf("%w: %s %w", indicator.ErrInvalidRequest, p.key, err)
		}
		if math.Trunc(value) != value || value < float64(p.minimum) || value > float64(p.maximum) {
			return nil, fmt.Errorf("%w: %s must be an integer between %d and %d", indicator.ErrInvalidRequest, p.key, p.minimum, p.maximum)
		}
		values[index] = int(value)
	}
	return values, nil
}
