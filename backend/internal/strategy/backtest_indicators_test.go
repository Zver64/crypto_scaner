package strategy

import (
	"context"
	"log/slog"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/pivot"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

func snapshotTestService(t *testing.T, store *backtestStore) *Service {
	t.Helper()
	registry := testRegistry(t)
	// No configured rows or writes: Preview must resolve all dependencies.
	source := testScannerIndicators(t, registry)
	service, err := NewService(store, source, registry, testDrawers(t, registry), noTracker{}, slog.New(slog.DiscardHandler), 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestBacktestSnapshotAndSignalOutputsOfUnconfiguredDependencies(t *testing.T) {
	var hourly, daily []market.Candle
	for i := range 3000 {
		hourly = append(hourly, testCandle(1, market.IntervalHour, backtestHour(i), 100+10*math.Sin(float64(i)/7)))
	}
	for i := range 160 {
		daily = append(daily, testCandle(1, market.IntervalDay, backtestStart.AddDate(0, 0, i-30), 100+float64(i%17)))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalDay] = map[int64][]market.Candle{1: daily}
	item := Strategy{ID: testID(1), Signal: true, Direction: DirectionLong, TargetRatio: 2, Window: 6,
		Expression: `close("1h") > 101 && Prev(bbands_upperband("1h", 10)) > 0 && Percentile(rsi("1h"), 3, 50) > 0 && ema("1d", 3) > 0 && !(pivot_low("1h") > 0) && Of("BTCUSDT", ema("1h", 7)) > 0`}
	store.strategies = []Strategy{item}
	service := snapshotTestService(t, store)
	// A bounded run must not return the latest stored tail or future indicators.
	to := hourly[2800].OpenTime
	result, err := service.Backtest(context.Background(), testID(1), "BTCUSDT", time.Time{}, to, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Charts) == 0 || len(result.Charts[0].Candles) <= market.SyncDepth || !result.To.Equal(to) {
		t.Fatalf("snapshot range: from %v to %v, chart count %d", result.From, result.To, len(result.Charts))
	}
	if !result.Charts[0].Candles[0].OpenTime.Equal(result.From) || !result.Charts[0].Candles[len(result.Charts[0].Candles)-1].OpenTime.Equal(result.To) {
		t.Fatal("chart differs from evaluated period")
	}
	keys := make(map[string]bool)
	for _, column := range result.IndicatorColumns {
		keys[column.Key] = true
		if column.Key == `close("1h")` || column.Key == `ema("1h", 7)` {
			t.Fatalf("nonlocal or candle column: %s", column.Key)
		}
	}
	for _, name := range []string{`rsi("1h")`, `ema("1d", 3)`, `bbands_upperband("1h", 10)`, `bbands_middleband("1h", 10)`, `bbands_lowerband("1h", 10)`, `pivot_high("1h")`, `pivot_low("1h")`} {
		if !keys[name] {
			t.Fatalf("missing output %s", name)
		}
	}
	if result.Signal == nil || len(result.Signal.Occurrences) == 0 {
		t.Fatal("expected signals")
	}
	if slices.ContainsFunc(service.indicators.List(), func(entry scannerindicator.Entry) bool { return true }) {
		t.Fatal("test unexpectedly configured an indicator")
	}
	charted := chartedOf(t, service, item)
	for _, occurrence := range result.Signal.Occurrences {
		for name := range occurrence.IndicatorValues {
			if !keys[name] {
				t.Fatalf("unlisted output %s", name)
			}
		}
		if _, exists := occurrence.Values[`rsi("1h")`]; exists {
			t.Fatal("fixture must read RSI only within percentile")
		}
		if _, exists := occurrence.IndicatorValues[`rsi("1h")`]; !exists {
			t.Fatal("current RSI missing despite percentile dependency")
		}
		// The rule holds only where there is no pivot low: the empty value
		// compares as false and is left out of both maps.
		if _, exists := occurrence.IndicatorValues[`pivot_low("1h")`]; exists {
			t.Fatal("empty pivot low among the indicator values")
		}
		if _, exists := occurrence.Values[`pivot_low("1h")`]; exists {
			t.Fatal("empty pivot low among the read values")
		}
		// Current outputs at the signal and chart points must agree exactly.
		for _, page := range result.Charts {
			open := page.Interval.LastClosedOpenTime(result.Interval.NextOpenTime(occurrence.Time))
			for _, shape := range page.Shapes {
				item, output := chartedShape(t, charted, shape)
				if !slices.Contains(item.Outputs, output) {
					continue
				}
				name := item.column(output)
				var drawn []float64
				switch {
				case shape.Line != nil:
					for _, point := range shape.Line.Points {
						if point.Time.Equal(open) {
							drawn = append(drawn, point.Value)
						}
					}
				case shape.Markers != nil:
					for _, marker := range shape.Markers.Points {
						if marker.Calculated.Equal(open) {
							drawn = append(drawn, marker.Value)
						}
					}
				}
				for _, value := range drawn {
					if table, known := occurrence.IndicatorValues[name]; !known || table != value {
						t.Fatalf("%s at %v: table %v (%v), chart %v", name, occurrence.Time, table, known, value)
					}
				}
			}
		}
	}
	for _, page := range result.Charts {
		for _, shape := range page.Shapes {
			item, _ := chartedShape(t, charted, shape)
			if item.Selection.Type == "ema" && page.Interval == market.IntervalHour {
				t.Fatal("of-only indicator drawn")
			}
			// Pivots are markers only on the candles confirming them.
			if item.Selection.Type == pivot.Type && shape.Markers != nil && (len(shape.Markers.Points) == 0 || len(shape.Markers.Points) > len(page.Candles)/10) {
				t.Fatalf("%s has %d markers over %d candles", shape.ID, len(shape.Markers.Points), len(page.Candles))
			}
		}
		for _, candle := range page.Candles {
			if candle.OpenTime.Before(page.Interval.OpenTime(result.From)) || page.Interval.NextOpenTime(candle.OpenTime).After(result.Interval.NextOpenTime(result.To)) {
				t.Fatalf("candle outside snapshot: %v", candle.OpenTime)
			}
		}
	}
	// The draft path has the same columns and no graph payload unless requested.
	draft, err := service.BacktestDraft(context.Background(), item, "BTCUSDT", result.From, to, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Charts) != 0 || !slices.Equal(draft.IndicatorColumns, result.IndicatorColumns) {
		t.Fatal("draft dependencies differ or unsolicited chart")
	}
	// Correcting stored history after a run cannot mutate returned candles.
	before := result.Charts[0].Candles[0].Close
	first := slices.IndexFunc(store.candles[market.IntervalHour][1], func(c market.Candle) bool { return c.OpenTime.Equal(result.From) })
	store.candles[market.IntervalHour][1][first].Close = 1
	if result.Charts[0].Candles[0].Close != before {
		t.Fatal("snapshot shares mutable candles")
	}
}

// A chart draws the source of an indicator once, though the rule does not
// read it by itself: the markers of its pivots sit on its line, and an
// indicator in its units shares its pane.
func TestBacktestChartDrawsTheSourcesOfIndicators(t *testing.T) {
	var hourly []market.Candle
	for i := range 600 {
		hourly = append(hourly, testCandle(1, market.IntervalHour, backtestHour(i), 100+10*math.Sin(float64(i)/7)))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	item := Strategy{ID: testID(1), Signal: true, Direction: DirectionLong, TargetRatio: 2, Window: 6,
		Expression: `pivot_low("1h", 5, 5, rsi("1h")) > 0 && ema("1h", 9, macd_macdhist("1h")) > -100`}
	store.strategies = []Strategy{item}
	service := snapshotTestService(t, store)
	result, err := service.Backtest(context.Background(), testID(1), "BTCUSDT", time.Time{}, time.Time{}, true)
	if err != nil {
		t.Fatal(err)
	}
	page := result.Charts[0]
	charted := chartedOf(t, service, item)
	// places are where the shapes of each type lie, by output.
	places := map[indicator.Type]map[string]chart.Place{}
	for _, shape := range page.Shapes {
		drawn, output := chartedShape(t, charted, shape)
		if places[drawn.Selection.Type] == nil {
			places[drawn.Selection.Type] = map[string]chart.Place{}
		}
		if _, twice := places[drawn.Selection.Type][output]; twice {
			t.Fatalf("%s %s is drawn twice", drawn.Selection.Type, output)
		}
		places[drawn.Selection.Type][output] = shape.Place
		if drawn.Selection.Type == "rsi" && len(shape.Line.Points) == 0 {
			t.Fatal("the source has no points")
		}
		// The markers of the pivots sit on the line of the rsi.
		if drawn.Selection.Type == pivot.Type && shape.Markers != nil && slices.ContainsFunc(shape.Markers.Points, func(marker chart.Marker) bool { return marker.LineValue == nil }) {
			t.Fatalf("pivot %s has markers off the rsi line", output)
		}
	}
	for _, name := range []indicator.Type{"rsi", "macd", "ema", pivot.Type} {
		if places[name] == nil {
			t.Fatalf("%s is not drawn among %v", name, places)
		}
	}
	rsi := places["rsi"]["rsi"]
	if rsi.Placement != chart.PlacementPane || places[pivot.Type]["high"] != rsi || places[pivot.Type]["low"] != rsi {
		t.Fatalf("pivots of the rsi lie in %+v, want the pane of the rsi, %+v", places[pivot.Type], rsi)
	}
	// The pivot lows the signal fired on are joined to the ones before, on
	// the rsi line and on the candles.
	if places[pivot.Type]["low-connected"] != rsi || places[pivot.Type]["low-connected-candles"] != chart.PriceScale {
		t.Fatalf("pivot shapes lie in %+v, want the signal pivots connected on the rsi line and the candles", places[pivot.Type])
	}
	if macd, ema := places["macd"]["macdhist"], places["ema"]["ema"]; ema != macd || macd.Pane == rsi.Pane {
		t.Fatalf("ema of the macd histogram lies in %+v, want the own pane of the macd, %+v", ema, macd)
	}
	// Only what the rule reads is a column.
	if slices.ContainsFunc(result.IndicatorColumns, func(column IndicatorColumn) bool { return column.Key == `rsi("1h")` }) {
		t.Fatal("the source is a column")
	}
}

func TestBacktestDependenciesIncludeExitAndPriceRules(t *testing.T) {
	service := snapshotTestService(t, newBacktestStore(market.IntervalHour, nil))
	entry, err := service.compileEntry(Strategy{
		Expression: `rsi("1h") > 30`, ExitExpression: `ema("1d", 3) > close("1d")`,
		TakeProfitExpression: `bbands_upperband("1h", 10)`, StopLossExpression: `close("1h") - atr("1h")`,
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.displayedIndicators(entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, read := range entry.reads {
		if read.Variable.Target.Equal(CandleTarget(read.Variable.Target.Interval())) {
			continue
		}
		if !slices.ContainsFunc(items, func(item displayedIndicator) bool { return item.target().Equal(read.Variable.Target) }) {
			t.Fatalf("dependency missing: %s", read.Variable.Name)
		}
	}
}

// chartedOf are the indicators the backtest chart of item draws.
func chartedOf(t *testing.T, service *Service, item Strategy) []displayedIndicator {
	t.Helper()
	entry, err := service.compileEntry(item)
	if err != nil {
		t.Fatal(err)
	}
	displayed, err := service.displayedIndicators(entry)
	if err != nil {
		t.Fatal(err)
	}
	charted, err := service.chartedIndicators(displayed)
	if err != nil {
		t.Fatal(err)
	}
	return charted
}

// chartedShape returns the indicator among charted a shape draws, which
// names the shape after its target, and the rest of the shape id, such as
// the output of a line.
func chartedShape(t *testing.T, charted []displayedIndicator, shape chart.Shape) (displayedIndicator, string) {
	t.Helper()
	cut := strings.LastIndex(shape.ID, "/")
	index := slices.IndexFunc(charted, func(item displayedIndicator) bool { return cut >= 0 && item.target().Key() == shape.ID[:cut] })
	if index < 0 {
		t.Fatalf("shape %s draws no charted indicator", shape.ID)
	}
	return charted[index], shape.ID[cut+1:]
}
