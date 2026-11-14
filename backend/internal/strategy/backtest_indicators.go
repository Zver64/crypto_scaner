package strategy

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
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

// displayedIndicator is an indicator a backtest reads on interval, whose
// calls take arguments after the period.
type displayedIndicator struct {
	scannerindicator.Entry
	interval  market.CandleInterval
	arguments []string
}

// column names an output of the indicator as expressions read it, such as
// rsi("1h") or bbands_upperband("1h", 20, 2.5).
func (item displayedIndicator) column(output string) string {
	return spellCall(indicatorFunction(item.Selection.Type, len(item.Outputs), output), item.interval, item.arguments)
}

func (item displayedIndicator) target() closedindicator.Target {
	return item.Target(item.interval)
}

func (service *Service) displayedIndicators(entry Entry) ([]displayedIndicator, error) {
	var result []displayedIndicator
	for _, read := range entry.reads {
		target := read.Variable.Target
		if read.Symbol != "" || read.Variable.Position || target.Equal(CandleTarget(target.Interval)) ||
			slices.ContainsFunc(result, func(item displayedIndicator) bool { return item.target().Equal(target) }) {
			continue
		}
		// Derive presentation from the dependency itself, never from configured
		// rows, their display flags, order, or scale overrides.
		item, err := service.indicators.Preview(target.Selection)
		if err != nil {
			return nil, fmt.Errorf("describe backtest indicator: %w", err)
		}
		arguments, err := callArguments(service.registry, target.Selection)
		if err != nil {
			return nil, fmt.Errorf("spell backtest indicator: %w", err)
		}
		result = append(result, displayedIndicator{Entry: item, interval: target.Interval, arguments: arguments})
	}
	// Formula traversal contains maps. Canonical calls give both clients a stable order,
	// independent of configured indicator order and display flags.
	slices.SortFunc(result, func(a, b displayedIndicator) int {
		return strings.Compare(a.column(a.Outputs[0]), b.column(b.Outputs[0]))
	})
	return result, nil
}

func indicatorColumns(items []displayedIndicator) []IndicatorColumn {
	columns := make([]IndicatorColumn, 0)
	for _, item := range items {
		for _, output := range item.Outputs {
			name := item.column(output)
			columns = append(columns, IndicatorColumn{Key: name, Title: name})
		}
	}
	return columns
}

func (current snapshot) indicatorValues(entry Entry, instrumentID int64, items []displayedIndicator) map[string]float64 {
	result := make(map[string]float64)
	for _, item := range items {
		position, found := current.reads.position(entry.ID, instrumentID, instrumentID, item.target().Key())
		if !found {
			continue
		}
		value := current.values[position]
		if !value.OpenTime.Equal(item.interval.LastClosedOpenTime(current.now)) {
			continue
		}
		for _, output := range value.Outputs {
			if shown, ok := output.Shown(); ok {
				result[item.column(output.Name)] = shown
			}
		}
	}
	return result
}

// chartSnapshot uses the same per-candle windows as replay. Values already
// calculated by replay are reused; gaps in the base interval do not lose points
// of coarser intervals. No snapshot reads history again after the replay.
func (replay *replay) chartSnapshot(ctx context.Context, entry Entry, instrumentID int64, result Backtest) ([]BacktestChart, error) {
	charts := make([]BacktestChart, 0)
	if result.From.IsZero() {
		return charts, nil
	}
	lastExit := result.To
	if result.Signal != nil && result.Direction != DirectionSideways {
		for _, occurrence := range result.Signal.Occurrences {
			if evaluation := occurrence.Evaluation; evaluation != nil && evaluation.ExitTime.After(lastExit) {
				lastExit = evaluation.ExitTime
			}
		}
	}
	end := result.Interval.NextOpenTime(lastExit)
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
			if item.interval != interval {
				continue
			}
			definition := item.ChartIndicator(item.target().Key(), line)
			line += len(definition.Lines)
			page.Definitions = append(page.Definitions, definition)
			calculation := indicator.Calculation{Type: item.Selection.Type, Parameters: item.Selection.Parameters, Series: make([]indicator.NamedSeries, len(definition.Lines))}
			for i, drawn := range definition.Lines {
				calculation.Series[i] = indicator.NamedSeries{Name: drawn.Output, Points: make([]indicator.Point, 0)}
			}
			target := item.target()
			position, subscribed := replay.current.reads.position(entry.ID, instrumentID, instrumentID, target.Key())
			if !subscribed {
				return nil, fmt.Errorf("backtest chart target %s is not subscribed", target.Key())
			}
			cached := replay.chartValues[pair{instrumentID: instrumentID, target: target.Key()}]
			for i := first; first >= 0 && i <= last; i++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				candle := history[i]
				outputs, found := cached[candle.OpenTime.UTC()]
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
					shown, ok := output.Shown()
					if !ok {
						continue
					}
					for j := range calculation.Series {
						if calculation.Series[j].Name == output.Name {
							calculation.Series[j].Points = append(calculation.Series[j].Points, indicator.Point{Time: candle.OpenTime.UTC(), Value: shown})
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
