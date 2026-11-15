package pivot

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"crypto-scanner/internal/indicator"
)

const (
	// DivergenceType is the registry identifier of the RSI divergences.
	DivergenceType indicator.Type = "divergence"
	// rsiPeriod is the RSI of TradingView's RSI Divergence Indicator.
	rsiPeriod = 14
	// minBetween and maxBetween bound the candles between two pivots that
	// form a divergence.
	minBetween, maxBetween = 5, 60
	// pivotsKey is the parameter choosing the series the pivots are found
	// in: the RSI (pivotsRSI, the default) or the price (pivotsPrice), the
	// low for pivot lows and the high for pivot highs.
	pivotsKey              = "pivots"
	pivotsRSI, pivotsPrice = 0, 1
)

// pivotsChoices are the values of the pivots parameter, which calls write by
// name.
var pivotsChoices = []indicator.Choice{
	{Value: pivotsRSI, Title: "RSI", Name: "rsi"},
	{Value: pivotsPrice, Title: "Price", Name: "price"},
}

// NewDivergence returns the regular RSI divergences of TradingView's RSI
// Divergence Indicator. rsi is the registered RSI module, which calculates
// the RSI 14 of the close, so the RSI is the one rsi("1h") reads.
func NewDivergence(rsi indicator.Implementation) indicator.Implementation {
	return divergences{rsi: rsi}
}

// divergences reports, on the candle that confirms a pivot low, the RSI at
// that pivot when the RSI makes a higher low and the price a lower low than
// at the pivot low before (bull), and likewise for pivot highs, a lower RSI
// high and a higher price high (bear), when 5 to 60 candles lie between the
// two pivots; both are empty on every other candle. The pivots are those of
// the RSI, or of the price (pivot lows of the low and pivot highs of the
// high). Each is the rule over pivot_low and pivot_high of rsi("1h"), or of
// the price, in docs/strategy-language.md. bull_bars and bear_bars hold,
// beside them, the candles from the earlier pivot to the later one, where
// charts start the line to the marker.
type divergences struct {
	rsi indicator.Implementation
}

func (divergences) Describe() indicator.Descriptor {
	return indicator.Descriptor{
		Type: DivergenceType, Title: "RSI Divergence", Group: group, Unstable: true,
		Inputs: []string{"close", "high", "low"},
		Parameters: append(describe(sides), indicator.ParameterDescriptor{
			Key: pivotsKey, Title: "Pivots in", Description: "Series the pivots are found in: the RSI, or the low and the high", Kind: indicator.ParameterChoice,
			Default: pivotsRSI, Choices: slices.Clone(pivotsChoices),
		}),
		Outputs: []indicator.OutputDescriptor{
			{Name: "bull", Style: indicator.OutputMarkerBelow, Sparse: true},
			{Name: "bear", Style: indicator.OutputMarkerAbove, Sparse: true},
			{Name: "rsi", Style: indicator.OutputLine},
			{Name: "bull_bars", Style: indicator.OutputValue, Sparse: true},
			{Name: "bear_bars", Style: indicator.OutputValue, Sparse: true},
		},
	}
}

// parse reads the sides and the choice of the series the pivots are found
// in.
func (divergences) parse(parameters indicator.Parameters) (left, right, in int, err error) {
	in = pivotsRSI
	if raw, chosen := parameters[pivotsKey]; chosen {
		value, err := indicator.ParseNumber(raw)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("%w: %s %w", indicator.ErrInvalidRequest, pivotsKey, err)
		}
		if !slices.ContainsFunc(pivotsChoices, func(choice indicator.Choice) bool { return float64(choice.Value) == value }) {
			return 0, 0, 0, fmt.Errorf("%w: %s must be one of its choices", indicator.ErrInvalidRequest, pivotsKey)
		}
		in = int(value)
		parameters = maps.Clone(parameters)
		delete(parameters, pivotsKey)
	}
	values, err := parse(sides, parameters)
	if err != nil {
		return 0, 0, 0, err
	}
	return values[0], values[1], in, nil
}

func (m divergences) Normalize(parameters indicator.Parameters) (indicator.Parameters, error) {
	left, right, in, err := m.parse(parameters)
	if err != nil {
		return nil, err
	}
	result := canonical(sides, []int{left, right})
	result[pivotsKey] = in
	return result, nil
}

func (m divergences) Fields(parameters indicator.Parameters) ([]string, error) {
	if _, _, _, err := m.parse(parameters); err != nil {
		return nil, err
	}
	return []string{"close", "high", "low"}, nil
}

// Lookback is that of the RSI followed by that of its pivots, as for
// pivot_low("1h", left, right, rsi("1h")), so both see the same RSI. Pivots
// of the price are found among the candles with an RSI as well, so every
// pivot has one.
func (m divergences) Lookback(parameters indicator.Parameters) (int, error) {
	left, right, _, err := m.parse(parameters)
	if err != nil {
		return 0, err
	}
	lookback, err := m.rsi.Lookback(rsiParameters())
	if err != nil {
		return 0, err
	}
	return lookback + left + right, nil
}

func (m divergences) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	left, right, in, err := m.parse(parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, []string{"close", "high", "low"})
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", indicator.ErrInvalidRequest, err)
	}
	calculated, err := m.rsi.Calculate(rsiParameters(), indicator.Inputs{"source": series[0]})
	if err != nil {
		return indicator.Result{}, err
	}
	rsi := calculated.Outputs["rsi"]
	// The prices at the RSI values.
	start := min(rsi.Offset, len(series[0]))
	highs, lows := series[1][start:], series[2][start:]
	lowPivots, highPivots := rsi.Values, rsi.Values
	if in == pivotsPrice {
		lowPivots, highPivots = lows, highs
	}
	bull, bullBars := divergent(rsi, lows, lowPivots, left, right, isLow, bullish)
	bear, bearBars := divergent(rsi, highs, highPivots, left, right, isHigh, bearish)
	return indicator.Result{Outputs: indicator.Outputs{
		"bull": bull, "bear": bear, "rsi": rsi,
		"bull_bars": bullBars, "bear_bars": bearBars,
	}}, nil
}

// divergent holds, on the candle confirming each pivot of in, which is the
// RSI or prices, the RSI there when it diverges from the pivot before, which
// diverges tells from the RSI and the prices at both, and in bars the
// candles from that pivot before to it; nothing elsewhere. The rule's
// pivot > 0 needs no check: prices are positive, and the RSI is never
// negative, so a higher RSI low is positive, and so is an RSI high above the
// RSI of the candles after it.
func divergent(rsi indicator.Series, prices, in []float64, left, right int, pivot func(values []float64, at, left, right int) bool, diverges func(rsi, rsiBefore, price, priceBefore float64) bool) (result, bars indicator.Series) {
	result = indicator.Series{Offset: rsi.Offset + left + right, Values: []float64{}}
	for range max(0, len(rsi.Values)-left-right) {
		result.Values = append(result.Values, math.NaN())
	}
	bars = indicator.Series{Offset: result.Offset, Values: slices.Clone(result.Values)}
	pivots := confirmed(in, left, right, pivot)
	for index := 1; index < len(pivots); index++ {
		before, at := pivots[index-1], pivots[index]
		// The candles between the pivots are those between their
		// confirmations, which Prev(BarsSince(…), 1) counts on the later one
		// as TradingView's ta.barssince(plFound[1]) does.
		if between := at - before - 1; between < minBetween || between > maxBetween {
			continue
		}
		if diverges(rsi.Values[at], rsi.Values[before], prices[at], prices[before]) {
			result.Values[at-left] = rsi.Values[at]
			bars.Values[at-left] = float64(at - before)
		}
	}
	return result, bars
}

// bullish is a higher RSI low beside a lower price low; bearish a lower RSI
// high beside a higher price high.
func bullish(rsi, rsiBefore, low, lowBefore float64) bool {
	return rsi > rsiBefore && low < lowBefore
}

func bearish(rsi, rsiBefore, high, highBefore float64) bool {
	return rsi < rsiBefore && high > highBefore
}

func rsiParameters() indicator.Parameters { return indicator.Parameters{"period": rsiPeriod} }
