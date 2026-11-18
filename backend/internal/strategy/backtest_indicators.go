package strategy

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// IndicatorColumn names a current output, not a shifted read or function result.
type IndicatorColumn struct{ Key, Title string }

// BacktestChart is immutable history of one interval within the evaluated
// period: its candles and the shapes of the charted indicators over them, in
// the panes Panes lists.
type BacktestChart struct {
	Interval market.CandleInterval
	Candles  []market.Candle
	Shapes   []chart.Shape
	Panes    []chart.Pane
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

func (item displayedIndicator) target() indicator.Target {
	return item.Target(item.interval)
}

// source is the indicator the item reads as its source, on its interval.
func (item displayedIndicator) source() (indicator.Target, bool) {
	selection, found := item.Source()
	return indicator.NewTarget(item.interval, selection), found
}

func (service *Service) displayedIndicators(entry Entry) ([]displayedIndicator, error) {
	var result []displayedIndicator
	for _, read := range entry.reads {
		target := read.Variable.Target
		if read.Symbol != "" || read.Variable.Position || target.Equal(CandleTarget(target.Interval())) {
			continue
		}
		var err error
		if result, err = service.withDisplayed(result, target); err != nil {
			return nil, err
		}
	}
	sortDisplayed(result)
	return result, nil
}

// chartedIndicators are the indicators backtest charts draw: the displayed
// ones and, once each, the indicators they read as their sources, which
// their markers and panes name.
func (service *Service) chartedIndicators(displayed []displayedIndicator) ([]displayedIndicator, error) {
	result := slices.Clone(displayed)
	for _, item := range displayed {
		if source, found := item.source(); found {
			var err error
			if result, err = service.withDisplayed(result, source); err != nil {
				return nil, err
			}
		}
	}
	sortDisplayed(result)
	return result, nil
}

// withDisplayed adds the indicator of target to items unless they have it.
func (service *Service) withDisplayed(items []displayedIndicator, target indicator.Target) ([]displayedIndicator, error) {
	if slices.ContainsFunc(items, func(item displayedIndicator) bool { return item.target().Equal(target) }) {
		return items, nil
	}
	// Derive presentation from the dependency itself, never from configured
	// rows, their display flags, order, or scale overrides.
	item, err := service.indicators.Preview(target.Selection())
	if err != nil {
		return nil, fmt.Errorf("describe backtest indicator: %w", err)
	}
	arguments, err := callArguments(service.registry, target.Selection(), target.Interval())
	if err != nil {
		return nil, fmt.Errorf("spell backtest indicator: %w", err)
	}
	return append(items, displayedIndicator{Entry: item, interval: target.Interval(), arguments: arguments}), nil
}

// sortDisplayed orders items by their canonical calls: formula traversal
// contains maps, and the calls give both clients a stable order, independent
// of configured indicator order and display flags.
func sortDisplayed(items []displayedIndicator) {
	slices.SortFunc(items, func(a, b displayedIndicator) int {
		return strings.Compare(a.column(a.Outputs[0]), b.column(b.Outputs[0]))
	})
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

// chartRecording is what a backtest chart draws on its instrument: the
// charted indicators and the values the replay calculated for the displayed
// ones while the backtest ran, so drawing does not calculate them again.
type chartRecording struct {
	instrumentID int64
	// charted are the indicators the chart draws: displayed and their
	// sources.
	charted []displayedIndicator
	// positions are the subscriptions of the replay that calculate a
	// displayed indicator on the instrument, and recorded the open time of
	// the value recorded last for each.
	positions []int
	recorded  []time.Time
	// values are the recorded outputs by target key and UTC open time.
	values map[string]map[time.Time][]closedindicator.Output
}

// chartRecording prepares the chart of the backtest replay runs of entry on
// instrumentID, which displays displayed: it gives replay the stored
// candles of every interval of the chart it lacks.
func (service *Service) chartRecording(ctx context.Context, replay *replay, entry Entry, instrumentID int64, displayed []displayedIndicator) (*chartRecording, error) {
	charted, err := service.chartedIndicators(displayed)
	if err != nil {
		return nil, err
	}
	recording := &chartRecording{instrumentID: instrumentID, charted: charted, values: map[string]map[time.Time][]closedindicator.Output{}}
	for position, subscription := range replay.current.reads.subscriptions {
		if subscription.InstrumentID == instrumentID && slices.ContainsFunc(displayed, func(item displayedIndicator) bool { return item.target().Equal(subscription.Target) }) {
			recording.positions = append(recording.positions, position)
		}
	}
	recording.recorded = make([]time.Time, len(recording.positions))
	for _, interval := range market.CandleIntervals() {
		if slices.Index(market.CandleIntervals(), interval) < slices.Index(market.CandleIntervals(), entry.Interval) {
			continue
		}
		if replay.histories[interval] == nil {
			replay.histories[interval] = map[int64][]market.Candle{}
		}
		if _, loaded := replay.histories[interval][instrumentID]; loaded {
			continue
		}
		stored, err := service.store.ListLatestCandles(ctx, []int64{instrumentID}, interval, market.RetentionDepth)
		if err != nil {
			return nil, fmt.Errorf("load backtest chart history: %w", err)
		}
		replay.histories[interval][instrumentID] = stored[instrumentID]
	}
	return recording, nil
}

// record keeps the values replay calculated last for the displayed
// indicators, those its window was warm for.
func (recording *chartRecording) record(replay *replay) {
	for index, position := range recording.positions {
		value := replay.current.values[position]
		if replay.cut[position] || value.OpenTime.Equal(recording.recorded[index]) {
			continue
		}
		recording.recorded[index] = value.OpenTime
		key := value.Target.Key()
		if recording.values[key] == nil {
			recording.values[key] = map[time.Time][]closedindicator.Output{}
		}
		outputs := make([]closedindicator.Output, len(value.Outputs))
		for i, output := range value.Outputs {
			outputs[i] = closedindicator.Output{Name: output.Name, Value: output.Value}
		}
		recording.values[key][value.OpenTime.UTC()] = outputs
	}
}

// pages draws the chart of result, using the same per-candle windows as
// replay. Values recorded while the backtest ran are reused; gaps in the
// base interval do not lose points of coarser intervals. A source the rule does not read by itself is
// calculated over the windows a read of its latest value would have. No
// snapshot reads history again after the replay. Every output is calculated,
// and charts draws them as it draws live charts.
func (recording *chartRecording) pages(ctx context.Context, charts ChartDrawing, replay *replay, entry Entry, result Backtest) ([]BacktestChart, error) {
	instrumentID := recording.instrumentID
	pages := make([]BacktestChart, 0)
	if result.From.IsZero() {
		return pages, nil
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
		page := BacktestChart{Interval: interval, Candles: make([]market.Candle, 0)}
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
		indicators := make([]chart.Calculated, 0)
		for _, item := range recording.charted {
			if item.interval != interval {
				continue
			}
			calculation := indicator.Calculation{Type: item.Selection.Type, Parameters: item.Selection.Parameters, Series: make([]indicator.NamedSeries, len(item.Outputs))}
			for j, output := range item.Outputs {
				calculation.Series[j] = indicator.NamedSeries{Name: output, Points: make([]indicator.Point, 0)}
			}
			target := item.target()
			subscription := closedindicator.Subscription{InstrumentID: instrumentID, Target: target, Points: 1}
			depth, err := closedindicator.Depth(replay.registry, target, subscription.Points)
			if err != nil {
				return nil, err
			}
			if position, subscribed := replay.current.reads.position(entry.ID, instrumentID, instrumentID, target.Key()); subscribed {
				subscription, depth = replay.current.reads.subscriptions[position], replay.depths[position]
			}
			cached := recording.values[target.Key()]
			for i := first; first >= 0 && i <= last; i++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				candle := history[i]
				outputs, found := cached[candle.OpenTime.UTC()]
				if !found {
					value, warm, err := replay.calculate(subscription, depth, i+1)
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
					j := slices.Index(item.Outputs, output.Name)
					if !ok || j < 0 {
						continue
					}
					calculation.Series[j].Points = append(calculation.Series[j].Points, indicator.Point{Time: candle.OpenTime.UTC(), Value: shown})
				}
			}
			indicators = append(indicators, chart.Calculated{ID: target.Key(), Calculation: calculation})
		}
		drawing, err := charts.Draw(page.Candles, indicators)
		if err != nil {
			return nil, fmt.Errorf("draw backtest chart: %w", err)
		}
		page.Shapes, page.Panes = drawing.Shapes, drawing.Panes
		// The markers a signal fired on are joined to the markers before.
		if result.Signal != nil {
			times := make([]time.Time, len(result.Signal.Occurrences))
			for i, occurrence := range result.Signal.Occurrences {
				times[i] = interval.OpenTime(occurrence.Time)
			}
			page.Shapes = append(page.Shapes, chart.Connectors(page.Shapes, page.Candles, times)...)
		}
		pages = append(pages, page)
	}
	return pages, nil
}
