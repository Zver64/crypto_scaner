package indicator

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"crypto-scanner/internal/market"
)

// CandleFields are the candle fields indicators can read, in display order.
var CandleFields = []string{"open", "high", "low", "close", "volume", "quote_volume", "trades"}

// PriceField reports whether field is on the candle price scale.
func PriceField(field string) bool {
	switch field {
	case "open", "high", "low", "close":
		return true
	}
	return false
}

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

// CalculateCandles calculates named series over contiguous candle runs, so no
// warm-up spans a gap. The same method handles closed history and a private
// extension with the current candle.
func (r *Registry) CalculateCandles(interval market.CandleInterval, candles []market.Candle, selections []Selection) ([]Calculation, error) {
	history := NewHistory(interval, candles)
	results := make([]Calculation, 0, len(selections))
	for _, selection := range selections {
		outputs, err := r.CalculateRange(history, 0, len(candles), selection)
		if err != nil {
			return nil, err
		}
		series := make([]NamedSeries, len(outputs))
		for index, output := range outputs {
			count := 0
			for _, run := range output.Runs {
				count += len(run.Values)
			}
			points := make([]Point, 0, count)
			for _, run := range output.Runs {
				for i, value := range run.Values {
					points = append(points, Point{Time: candles[run.Offset+i].OpenTime.UTC(), Value: value})
				}
			}
			series[index] = NamedSeries{Name: output.Name, Points: points}
		}
		results = append(results, Calculation{Type: selection.Type, Parameters: selection.Parameters, Series: series})
	}
	return results, nil
}

// History is a chronological candle history of one interval prepared for
// calculations over many of its windows: its contiguous runs are found once,
// and each candle field is converted once, when first read. It is not safe
// for concurrent use, and its candles must not change.
type History struct {
	interval market.CandleInterval
	candles  []market.Candle
	// breaks are the ascending indexes of the candles that do not open one
	// interval after the candle before them.
	breaks []int
	// fields are the converted candle fields by name.
	fields map[string][]float64
}

// NewHistory prepares candles, chronological candles of interval.
func NewHistory(interval market.CandleInterval, candles []market.Candle) *History {
	history := &History{interval: interval, candles: candles, fields: map[string][]float64{}}
	for i := 1; i < len(candles); i++ {
		if !interval.NextOpenTime(candles[i-1].OpenTime).Equal(candles[i].OpenTime) {
			history.breaks = append(history.breaks, i)
		}
	}
	return history
}

// OpenTime is the open time of the candle at index.
func (history *History) OpenTime(index int) time.Time { return history.candles[index].OpenTime }

// field returns the values of the candle field name of every candle.
func (history *History) field(name string) []float64 {
	if values, ok := history.fields[name]; ok {
		return values
	}
	values := make([]float64, len(history.candles))
	for i, candle := range history.candles {
		switch name {
		case "open":
			values[i] = candle.Open
		case "high":
			values[i] = candle.High
		case "low":
			values[i] = candle.Low
		case "close":
			values[i] = candle.Close
		case "volume":
			values[i] = candle.Volume
		case "quote_volume":
			values[i] = candle.QuoteAssetVolume
		case "trades":
			values[i] = float64(candle.TradeCount)
		}
	}
	history.fields[name] = values
	return values
}

// OutputRuns are the values of one named output, one Series per contiguous
// run of the calculated candles, in order. The Offset of each is the index in
// the history of the candle of its first value.
type OutputRuns struct {
	Name string
	Runs []Series
}

// CalculateRange calculates selection over the candles of history from index
// from through to−1, each contiguous run on its own, so no warm-up spans a
// gap, and returns its outputs sorted by name. The values must not be
// modified.
func (r *Registry) CalculateRange(history *History, from, to int, selection Selection) ([]OutputRuns, error) {
	fields, err := r.Fields(selection)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("indicator %q has no inputs", selection.Type)
	}
	starts := []int{from}
	first, _ := slices.BinarySearch(history.breaks, from+1)
	for _, index := range history.breaks[first:] {
		if index >= to {
			break
		}
		starts = append(starts, index)
	}
	starts = append(starts, to)
	byName := map[string][]Series{}
	for run := 0; run+1 < len(starts); run++ {
		start, end := starts[run], starts[run+1]
		inputs := make(Inputs, len(fields))
		for _, field := range fields {
			inputs[field] = history.field(field)[start:end:end]
		}
		calculated, err := r.Calculate(Request{Type: selection.Type, Parameters: selection.Parameters, Inputs: inputs})
		if err != nil {
			return nil, err
		}
		for name, output := range calculated.Outputs {
			if len(output.Values) > 0 && (output.Offset >= end-start || len(output.Values) > end-start-output.Offset) {
				return nil, fmt.Errorf("indicator %q output %q exceeds candle input", selection.Type, name)
			}
			byName[name] = append(byName[name], Series{Offset: start + output.Offset, Values: output.Values})
		}
	}
	outputs := make([]OutputRuns, 0, len(byName))
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		outputs = append(outputs, OutputRuns{Name: name, Runs: byName[name]})
	}
	return outputs, nil
}
