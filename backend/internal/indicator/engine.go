package indicator

import (
	"fmt"
	"sort"
	"time"

	"crypto-scanner/internal/market"
)

// Selection identifies an algorithm and its parameters independently of a chart.
type Selection struct {
	Type       Type
	Parameters Parameters
}
type Point struct {
	Time  time.Time
	Value float64
}
type NamedSeries struct {
	Name   string
	Points []Point
}
type Calculation struct {
	Type       Type
	Parameters Parameters
	Series     []NamedSeries
}

// Engine calculates named series over contiguous candle runs. The same method
// handles closed history and a private extension with the current candle.
type Engine struct{ registry Calculator }

func NewEngine(registry Calculator) *Engine                  { return &Engine{registry: registry} }
func (e *Engine) Lookback(t Type, p Parameters) (int, error) { return e.registry.Lookback(t, p) }
func (e *Engine) Calculate(interval market.CandleInterval, candles []market.Candle, selections []Selection) ([]Calculation, error) {
	starts := []int{0}
	for i := 1; i < len(candles); i++ {
		if !interval.NextOpenTime(candles[i-1].OpenTime).Equal(candles[i].OpenTime) {
			starts = append(starts, i)
		}
	}
	starts = append(starts, len(candles))
	results := make([]Calculation, 0, len(selections))
	for _, selection := range selections {
		fields, err := e.registry.Inputs(selection.Type)
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("indicator %q has no inputs", selection.Type)
		}
		byName := map[string][]Point{}
		for run := 0; run+1 < len(starts); run++ {
			start, end := starts[run], starts[run+1]
			inputs := Inputs{}
			for _, field := range fields {
				values := make([]float64, end-start)
				for i := start; i < end; i++ {
					switch field {
					case "open":
						values[i-start] = candles[i].Open
					case "high":
						values[i-start] = candles[i].High
					case "low":
						values[i-start] = candles[i].Low
					case "close":
						values[i-start] = candles[i].Close
					case "volume":
						values[i-start] = candles[i].Volume
					case "quote_asset_volume":
						values[i-start] = candles[i].QuoteAssetVolume
					case "trade_count":
						values[i-start] = float64(candles[i].TradeCount)
					default:
						return nil, fmt.Errorf("indicator %q requires unsupported input %q", selection.Type, field)
					}
				}
				inputs[field] = values
			}
			calculated, err := e.registry.Calculate(Request{Type: selection.Type, Parameters: selection.Parameters, Inputs: inputs})
			if err != nil {
				return nil, err
			}
			for name, output := range calculated.Outputs {
				if _, ok := byName[name]; !ok {
					byName[name] = []Point{}
				}
				if len(output.Values) > 0 && (output.Offset >= end-start || len(output.Values) > end-start-output.Offset) {
					return nil, fmt.Errorf("indicator %q output %q exceeds candle input", selection.Type, name)
				}
				for i, value := range output.Values {
					byName[name] = append(byName[name], Point{Time: candles[start+output.Offset+i].OpenTime.UTC(), Value: value})
				}
			}
		}
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		series := make([]NamedSeries, 0, len(names))
		for _, name := range names {
			series = append(series, NamedSeries{Name: name, Points: byName[name]})
		}
		results = append(results, Calculation{Type: selection.Type, Parameters: selection.Parameters, Series: series})
	}
	return results, nil
}
