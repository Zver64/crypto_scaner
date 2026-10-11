package strategy

import (
	"context"
	"testing"

	"crypto-scanner/internal/market"
)

func TestSignalSnapshotIncludesExitAfterRequestedPeriod(t *testing.T) {
	quiet := ohlc(100, 100.1, 99.9, 100)
	history := signalHistory(quiet, quiet, quiet, quiet, quiet, quiet)
	store := newBacktestStore(market.IntervalHour, history)
	store.strategies = []Strategy{{ID: 1, Signal: true, Direction: DirectionLong, TargetRatio: 2, Window: 6, Expression: "h_close > 0"}}
	service := snapshotTestService(t, store)
	at := history[signalHistoryCandles].OpenTime
	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", at, at, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Signal == nil || len(result.Signal.Occurrences) != 1 {
		t.Fatalf("missing occurrence: %+v", result.Signal)
	}
	evaluation := result.Signal.Occurrences[0].Evaluation
	if evaluation == nil || evaluation.Outcome != SignalNeutral {
		t.Fatalf("unexpected evaluation: %+v", evaluation)
	}
	last := history[len(history)-1].OpenTime
	if !evaluation.ExitTime.Equal(last) || !result.From.Equal(at) || !result.To.Equal(at) {
		t.Fatal("exit or evaluated period changed")
	}
	page := result.Charts[0]
	if len(page.Candles) != 7 || !page.Candles[len(page.Candles)-1].OpenTime.Equal(last) {
		t.Fatal("exit candle missing from snapshot")
	}
	// The chart tail must not add signals or statistical evaluations.
	if result.Signal.All.Count != 1 || result.Signal.Signals.Count != 1 {
		t.Fatal("chart tail was evaluated as entries")
	}
	withoutChart, err := service.Backtest(context.Background(), 1, "BTCUSDT", at, at, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutChart.Charts) != 0 || !withoutChart.To.Equal(at) {
		t.Fatal("unsolicited chart or expanded report")
	}
}
