package pivot

import (
	"fmt"
	"slices"

	"crypto-scanner/internal/indicator"
)

const (
	// DivergenceType is the registry identifier of the RSI divergence.
	DivergenceType indicator.Type = "divergence"
	// maxRange bounds the candles between two pivots of a divergence.
	maxRange = 500
	// maxPeriod bounds the RSI period, so the lookback stays well within the
	// synchronized history.
	maxPeriod = 100
)

// divergenceParams are the inputs of TradingView's RSI Divergence Indicator,
// in its order and with its defaults.
var divergenceParams = slices.Concat(
	[]param{{key: "period", title: "RSI Period", description: "Number of period", minimum: 2, maximum: maxPeriod, defaultValue: 14}},
	sides,
	[]param{
		{key: "range_lower", title: "Min of Lookback Range", description: "Fewest candles between two pivots", minimum: 0, maximum: maxRange, defaultValue: 5},
		{key: "range_upper", title: "Max of Lookback Range", description: "Most candles between two pivots", minimum: 0, maximum: maxRange, defaultValue: 60},
	},
)

// NewDivergence returns the RSI divergence module, which calculates the RSI
// with rsi, the TA-Lib RSI module, so it matches the configured RSI.
func NewDivergence(rsi indicator.Implementation) indicator.Implementation {
	return divergence{rsi: rsi}
}

// divergence marks with 1 the candles confirming a divergence of the price
// from the RSI and is 0 elsewhere. Like TradingView, it finds pivots of the
// RSI, and compares the lows (or highs) of the price at those candles: a pivot
// is confirmed right candles after it, and divergences are only found between
// a pivot and the previous one with range_lower to range_upper candles
// between them.
type divergence struct {
	rsi indicator.Implementation
}

func (divergence) Describe() indicator.Descriptor {
	return indicator.Descriptor{
		Type: DivergenceType, Title: "RSI Divergence", Group: group, Unstable: true,
		Inputs:     []string{"close", "high", "low"},
		Parameters: describe(divergenceParams),
		Outputs: []indicator.OutputDescriptor{
			{Name: "bull", Style: indicator.OutputHistogram},
			{Name: "hidden_bull", Style: indicator.OutputHistogram},
			{Name: "bear", Style: indicator.OutputHistogram},
			{Name: "hidden_bear", Style: indicator.OutputHistogram},
		},
	}
}

// settings are the parsed parameters with the RSI lookback.
type settings struct {
	period, left, right, lower, upper, lookback int
}

// offset is the first candle with a value: a pivot confirmed there and the
// farthest previous pivot it is compared with both have RSI values on every
// side.
func (s settings) offset() int { return s.lookback + s.left + s.right + s.upper + 1 }

func (m divergence) parse(parameters indicator.Parameters) (settings, error) {
	values, err := parse(divergenceParams, parameters)
	if err != nil {
		return settings{}, err
	}
	result := settings{period: values[0], left: values[1], right: values[2], lower: values[3], upper: values[4]}
	if result.lower > result.upper {
		return settings{}, fmt.Errorf("%w: range_lower must not exceed range_upper", ErrInvalidRequest)
	}
	result.lookback, err = m.rsi.Lookback(rsiParameters(result))
	if err != nil {
		return settings{}, err
	}
	return result, nil
}

func rsiParameters(s settings) indicator.Parameters {
	return indicator.Parameters{"period": s.period}
}

func (m divergence) Normalize(parameters indicator.Parameters) (indicator.Parameters, error) {
	s, err := m.parse(parameters)
	if err != nil {
		return nil, err
	}
	return canonical(divergenceParams, []int{s.period, s.left, s.right, s.lower, s.upper}), nil
}

func (m divergence) Fields(parameters indicator.Parameters) ([]string, error) {
	if _, err := m.parse(parameters); err != nil {
		return nil, err
	}
	return []string{"close", "high", "low"}, nil
}

func (m divergence) Lookback(parameters indicator.Parameters) (int, error) {
	s, err := m.parse(parameters)
	if err != nil {
		return 0, err
	}
	return s.offset(), nil
}

func (m divergence) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	s, err := m.parse(parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, []string{"close", "high", "low"})
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	closes, highs, lows := series[0], series[1], series[2]
	flags := map[string][]float64{}
	for _, output := range []string{"bull", "hidden_bull", "bear", "hidden_bear"} {
		flags[output] = make([]float64, len(closes))
	}
	if len(closes) > s.lookback {
		rsi, err := m.rsi.Calculate(rsiParameters(s), indicator.Inputs{"close": closes})
		if err != nil {
			return indicator.Result{}, err
		}
		oscillator := rsi.Outputs["rsi"]
		// Each pair compares the price at two consecutive RSI pivots: regular
		// when the price makes the extreme the RSI does not, hidden when the
		// RSI makes the one the price does not. beyond reports that a lies
		// past b in the direction of the pivots.
		mark := func(pivots []int, prices []float64, regular, hidden string, beyond func(a, b float64) bool) {
			for index := 1; index < len(pivots); index++ {
				previous, current := pivots[index-1], pivots[index]
				// TradingView's _inRange(plFound[1]): the candles strictly
				// between the two pivots.
				if between := current - previous - 1; between < s.lower || between > s.upper {
					continue
				}
				rsiNow, rsiThen := oscillator.Values[current], oscillator.Values[previous]
				priceNow, priceThen := prices[oscillator.Offset+current], prices[oscillator.Offset+previous]
				confirmation := oscillator.Offset + current + s.right
				if beyond(priceNow, priceThen) && beyond(rsiThen, rsiNow) {
					flags[regular][confirmation] = 1
				}
				if beyond(priceThen, priceNow) && beyond(rsiNow, rsiThen) {
					flags[hidden][confirmation] = 1
				}
			}
		}
		below := func(a, b float64) bool { return a < b }
		above := func(a, b float64) bool { return a > b }
		mark(confirmed(oscillator.Values, s.left, s.right, isLow), lows, "bull", "hidden_bull", below)
		mark(confirmed(oscillator.Values, s.left, s.right, isHigh), highs, "bear", "hidden_bear", above)
	}
	outputs := indicator.Outputs{}
	for output, values := range flags {
		series := indicator.Series{Offset: s.offset(), Values: []float64{}}
		if len(values) > series.Offset {
			series.Values = values[series.Offset:]
		}
		outputs[output] = series
	}
	return indicator.Result{Outputs: outputs}, nil
}
