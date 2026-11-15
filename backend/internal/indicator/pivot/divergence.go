package pivot

import (
	"fmt"
	"math"

	"crypto-scanner/internal/indicator"
)

const (
	// DivergenceType is the registry identifier of the RSI divergences.
	DivergenceType indicator.Type = "divergence"
	// rsiPeriod is the RSI of TradingView's RSI Divergence Indicator.
	rsiPeriod = 14
	// minBetween and maxBetween bound the candles between two pivots of the
	// RSI that form a divergence.
	minBetween, maxBetween = 5, 60
)

// NewDivergence returns the regular RSI divergences of TradingView's RSI
// Divergence Indicator. rsi is the registered RSI module, which calculates
// the RSI 14 of the close, so the RSI is the one rsi("1h") reads.
func NewDivergence(rsi indicator.Implementation) indicator.Implementation {
	return divergences{rsi: rsi}
}

// divergences reports, on the candle that confirms a pivot low of the RSI,
// the RSI at that pivot when the RSI makes a higher low and the price a lower
// low than at the pivot low before (bull), and likewise for pivot highs, a
// lower RSI high and a higher price high (bear), when 5 to 60 candles lie
// between the two pivots; both are empty on every other candle. Each is the
// rule over pivot_low and pivot_high of rsi("1h") in docs/strategy-language.md.
type divergences struct {
	rsi indicator.Implementation
}

func (divergences) Describe() indicator.Descriptor {
	return indicator.Descriptor{
		Type: DivergenceType, Title: "RSI Divergence", Group: group, Unstable: true,
		Inputs:     []string{"close", "high", "low"},
		Parameters: describe(sides),
		Outputs: []indicator.OutputDescriptor{
			{Name: "bull", Style: indicator.OutputMarkerBelow, Sparse: true, Shift: "right", Anchor: "rsi"},
			{Name: "bear", Style: indicator.OutputMarkerAbove, Sparse: true, Shift: "right", Anchor: "rsi"},
			{Name: "rsi", Style: indicator.OutputLine},
		},
	}
}

func (divergences) Normalize(parameters indicator.Parameters) (indicator.Parameters, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return nil, err
	}
	return canonical(sides, values), nil
}

func (divergences) Fields(parameters indicator.Parameters) ([]string, error) {
	if _, err := parse(sides, parameters); err != nil {
		return nil, err
	}
	return []string{"close", "high", "low"}, nil
}

// Lookback is that of the RSI followed by that of its pivots, as for
// pivot_low("1h", left, right, rsi("1h")), so both see the same RSI.
func (m divergences) Lookback(parameters indicator.Parameters) (int, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return 0, err
	}
	lookback, err := m.rsi.Lookback(rsiParameters())
	if err != nil {
		return 0, err
	}
	return lookback + values[0] + values[1], nil
}

func (m divergences) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	values, err := parse(sides, parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	left, right := values[0], values[1]
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
	return indicator.Result{Outputs: indicator.Outputs{
		"bull": divergent(rsi, lows, left, right, isLow, bullish),
		"bear": divergent(rsi, highs, left, right, isHigh, bearish),
		"rsi":  rsi,
	}}, nil
}

// divergent holds, on the candle confirming each pivot of the RSI, its value
// when it diverges from the pivot before, which diverges tells from the RSI
// and the prices at both; nothing elsewhere. The rule's pivot > 0 needs no
// check: the RSI is never negative, so a higher low is positive, and so is a
// high above the RSI of the candles after it.
func divergent(rsi indicator.Series, prices []float64, left, right int, pivot func(values []float64, at, left, right int) bool, diverges func(rsi, rsiBefore, price, priceBefore float64) bool) indicator.Series {
	result := indicator.Series{Offset: rsi.Offset + left + right, Values: []float64{}}
	for range max(0, len(rsi.Values)-left-right) {
		result.Values = append(result.Values, math.NaN())
	}
	pivots := confirmed(rsi.Values, left, right, pivot)
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
		}
	}
	return result
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
