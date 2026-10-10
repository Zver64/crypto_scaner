package strategy

import (
	"context"
	"log/slog"
	"math"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/indicator/pivot"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

func snapshotTestService(t *testing.T, store *backtestStore) *Service {
	t.Helper()
	registry, err := indicator.NewRegistry(append(indicatortalib.New(), candle.New(), pivot.New())...)
	if err != nil {
		t.Fatal(err)
	}
	// No configured rows or writes: Preview must resolve all dependencies.
	source, err := scannerindicator.New(&struct{ scannerindicator.Store }{}, registry, []string{"yellow.5", "blue.5"}, slog.New(slog.DiscardHandler), func() {}, func() map[int64][]string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, source, registry, 7, slog.New(slog.DiscardHandler), func([]int64) {})
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
	item := Strategy{ID: 1, Signal: true, Direction: DirectionLong, TargetRatio: 2, Window: 6,
		Expression: `h_close > 101 && prev(h_bbands_20_upperband) > 0 && percentile(h_rsi, 3, 50) > 0 && d_ema_3 > 0 && h_pivot_low_bars > 0 && of("BTCUSDT", h_ema_7) > 0`}
	store.strategies = []Strategy{item}
	service := snapshotTestService(t, store)
	// A bounded run must not return the latest stored tail or future indicators.
	to := hourly[2800].OpenTime
	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, to, true)
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
		if column.Key == "h_close" || column.Key == "h_ema_7" {
			t.Fatalf("nonlocal or candle column: %s", column.Key)
		}
	}
	for _, name := range []string{"h_rsi", "d_ema_3", "h_bbands_20_upperband", "h_bbands_20_middleband", "h_bbands_20_lowerband", "h_pivot_low_bars"} {
		if !keys[name] {
			t.Fatalf("missing output %s", name)
		}
	}
	if result.Signal == nil || len(result.Signal.Occurrences) == 0 {
		t.Fatal("expected signals")
	}
	for _, occurrence := range result.Signal.Occurrences {
		for name := range occurrence.IndicatorValues {
			if !keys[name] {
				t.Fatalf("unlisted output %s", name)
			}
		}
		if _, exists := occurrence.Values["h_rsi"]; exists {
			t.Fatal("fixture must read RSI only within percentile")
		}
		if _, exists := occurrence.IndicatorValues["h_rsi"]; !exists {
			t.Fatal("current RSI missing despite percentile dependency")
		}
		// Current outputs at the signal and chart points must agree exactly.
		for _, page := range result.Charts {
			for j, definition := range page.Definitions {
				if definition.Selection.Type == "ema" && page.Interval == market.IntervalHour {
					t.Fatal("of-only indicator drawn")
				}
				for _, line := range definition.Lines {
					if line.Output == "low_bars" {
						t.Fatal("hidden count drawn")
					}
				}
				for _, series := range page.Indicators[j].Series {
					entry := slices.IndexFunc(service.indicators.List(), func(e scannerindicator.Entry) bool { return e.Target().Selection.Type == definition.Selection.Type })
					if entry >= 0 {
						t.Fatal("test unexpectedly configured an indicator")
					}
					preview, err := service.indicators.Preview(scannerindicator.Indicator{Interval: page.Interval, Selection: definition.Selection})
					if err != nil {
						t.Fatal(err)
					}
					name := outputName(preview, series.Name)
					open := page.Interval.LastClosedOpenTime(result.Interval.NextOpenTime(occurrence.Time))
					point := slices.IndexFunc(series.Points, func(p indicator.Point) bool { return p.Time.Equal(open) })
					if point >= 0 {
						value, known := occurrence.IndicatorValues[name]
						if !known || value != series.Points[point].Value {
							t.Fatalf("%s at %v: table %v (%v), chart %v", name, occurrence.Time, value, known, series.Points[point].Value)
						}
					}
				}
			}
		}
	}
	for _, page := range result.Charts {
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

func TestBacktestDependenciesIncludeExitAndPriceRules(t *testing.T) {
	service := snapshotTestService(t, newBacktestStore(market.IntervalHour, nil))
	entry, err := service.compileEntry(Strategy{
		Expression: "h_rsi > 30", ExitExpression: "d_ema_3 > d_close",
		TakeProfitExpression: "h_bbands_20_upperband", StopLossExpression: "h_close - h_atr",
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.displayedIndicators(entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, read := range entry.reads {
		if read.Variable.Target.Equal(CandleTarget(read.Variable.Target.Interval)) {
			continue
		}
		if !slices.ContainsFunc(items, func(item scannerindicator.Entry) bool { return item.Target().Equal(read.Variable.Target) }) {
			t.Fatalf("dependency missing: %s", read.Variable.Name)
		}
	}
}
