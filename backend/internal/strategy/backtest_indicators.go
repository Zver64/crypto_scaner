package strategy

import (
	"context"
	"fmt"
	"slices"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// IndicatorColumn names a current output, not a shifted read or function result.
type IndicatorColumn struct{ Key, Title string }

// BacktestChart is immutable history of one interval within the evaluated period.
type BacktestChart struct {
	Interval    market.CandleInterval
	Candles     []market.Candle
	Definitions []chart.CatalogIndicator
	Indicators  []indicator.Calculation
}

func (service *Service) displayedIndicators(entry Entry) ([]scannerindicator.Entry, error) {
	var result []scannerindicator.Entry
	for _, read := range entry.reads {
		target := read.Variable.Target
		if read.Symbol != "" || read.Variable.Position || target.Equal(CandleTarget(target.Interval)) ||
			slices.ContainsFunc(result, func(item scannerindicator.Entry) bool { return item.Target().Equal(target) }) {
			continue
		}
		// Derive presentation from the dependency itself, never from configured
		// rows, their display flags, order, or scale overrides.
		item, err := service.indicators.Preview(scannerindicator.Indicator{Interval: target.Interval, Selection: target.Selection})
		if err != nil {
			return nil, fmt.Errorf("describe backtest indicator: %w", err)
		}
		result = append(result, item)
	}
	// Formula traversal contains maps. Canonical titles give both clients a stable order,
	// independent of configured indicator order and display flags.
	slices.SortFunc(result, func(a, b scannerindicator.Entry) int {
		if a.Title < b.Title {
			return -1
		}
		if a.Title > b.Title {
			return 1
		}
		return 0
	})
	return result, nil
}

func indicatorColumns(items []scannerindicator.Entry) []IndicatorColumn {
	columns := make([]IndicatorColumn, 0)
	for _, item := range items {
		for _, output := range item.Outputs {
			name := outputName(item, output)
			columns = append(columns, IndicatorColumn{Key: name, Title: name})
		}
	}
	return columns
}

func (current snapshot) indicatorValues(instrumentID int64, items []scannerindicator.Entry) map[string]float64 {
	result := make(map[string]float64)
	for _, item := range items {
		position, found := current.reads.positions[pair{instrumentID, item.Target().Key()}]
		if !found {
			continue
		}
		value := current.values[position]
		if !value.OpenTime.Equal(item.Interval.LastClosedOpenTime(current.now)) {
			continue
		}
		for _, output := range value.Outputs {
			result[outputName(item, output.Name)] = output.Value
		}
	}
	return result
}

// chartSnapshot uses the same per-candle windows as replay. Values already
// calculated by replay are reused; gaps in the base interval do not lose points
// of coarser intervals. No snapshot reads history again after the replay.
func (replay *replay) chartSnapshot(ctx context.Context, instrumentID int64, result Backtest) ([]BacktestChart, error) {
	charts := make([]BacktestChart, 0)
	if result.From.IsZero() {
		return charts, nil
	}
	end := result.Interval.NextOpenTime(result.To)
	for _, interval := range market.CandleIntervals() {
		if slices.Index(market.CandleIntervals(), interval) < slices.Index(market.CandleIntervals(), result.Interval) {
			continue
		}
		history := replay.histories[interval][instrumentID]
		page := BacktestChart{Interval: interval, Candles: make([]market.Candle, 0), Definitions: make([]chart.CatalogIndicator, 0), Indicators: make([]indicator.Calculation, 0)}
		first, last := -1, -1
		for i, candle := range history {
			if interval.NextOpenTime(candle.OpenTime).After(end) {
				break
			}
			if candle.OpenTime.Before(interval.OpenTime(result.From)) {
				continue
			}
			if first < 0 {
				first = i
			}
			last = i
			page.Candles = append(page.Candles, candle)
		}
		line := 0
		for _, item := range replay.displayed {
			if item.Interval != interval {
				continue
			}
			definition := item.ChartIndicator(item.Target().Key(), line)
			line += len(definition.Lines)
			page.Definitions = append(page.Definitions, definition)
			calculation := indicator.Calculation{Type: item.Selection.Type, Parameters: item.Selection.Parameters, Series: make([]indicator.NamedSeries, len(definition.Lines))}
			for i, drawn := range definition.Lines {
				calculation.Series[i] = indicator.NamedSeries{Name: drawn.Output, Points: make([]indicator.Point, 0)}
			}
			target := item.Target()
			position, subscribed := replay.current.reads.positions[pair{instrumentID, target.Key()}]
			if !subscribed {
				return nil, fmt.Errorf("backtest chart target %s is not subscribed", target.Key())
			}
			cached := replay.chartValues[fmt.Sprintf("%d|%s", instrumentID, target.Key())]
			for i := first; first >= 0 && i <= last; i++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				candle := history[i]
				outputs, found := cached[candle.OpenTime]
				if !found {
					value, warm, err := replay.calculateAt(position, i+1)
					if err != nil {
						return nil, err
					}
					if !warm {
						continue
					}
					outputs = value.Outputs
				}
				for _, output := range outputs {
					for j := range calculation.Series {
						if calculation.Series[j].Name == output.Name {
							calculation.Series[j].Points = append(calculation.Series[j].Points, indicator.Point{Time: candle.OpenTime.UTC(), Value: output.Value})
						}
					}
				}
			}
			page.Indicators = append(page.Indicators, calculation)
		}
		charts = append(charts, page)
	}
	return charts, nil
}
