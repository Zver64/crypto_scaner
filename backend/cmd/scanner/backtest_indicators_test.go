package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/apiclient"
)

func TestBacktestIndicatorFlagAndCappedSignals(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	occurrences := make([]apiclient.BacktestSignalOccurrence, maxTradeRows+3)
	for i := range occurrences {
		occurrences[i] = apiclient.BacktestSignalOccurrence{Time: from.Add(time.Duration(i) * time.Hour), Close: 100,
			IndicatorValues: map[string]float64{"h_rsi": float64(i), "h_pivot_low_bars": 0}}
	}
	backtest := apiclient.StrategyBacktest{
		From: &from, To: &occurrences[len(occurrences)-1].Time, Symbol: "BTCUSDT", Interval: apiclient.N1h, Direction: apiclient.Long,
		IndicatorColumns: []apiclient.BacktestIndicatorColumn{{Key: "h_rsi", Title: "h_rsi"}, {Key: "h_pivot_low_bars", Title: "h_pivot_low_bars"}, {Key: "h_pivot_high_bars", Title: "h_pivot_high_bars"}},
		Signal:           &apiclient.BacktestSignal{Window: 6, TargetRatio: 2, Occurrences: occurrences},
	}
	body, err := json.Marshal(backtest)
	if err != nil {
		t.Fatal(err)
	}
	_, server := newFakeAPI(t, map[string]string{"GET /api/v1/admin/strategies/3/backtest?symbol=BTCUSDT": string(body)})
	home := writeProfile(t, server.URL)
	for _, indicators := range []bool{false, true} {
		args := []string{"backtest", "--strategy", "3", "--symbol", "BTCUSDT"}
		if indicators {
			args = append(args, "--indicators")
		}
		got := runCLI(t, home, "", args...)
		if got.code != 0 || got.stderr != "" {
			t.Fatalf("run: %+v", got)
		}
		if strings.Contains(got.stdout, "H_RSI") != indicators || strings.Contains(got.stdout, "H_PIVOT_LOW_BARS") != indicators {
			t.Fatalf("indicator columns: %s", got.stdout)
		}
		// The parameter summary still mentions the beginning of the period,
		// so inspect just the occurrence table for omitted older candles.
		table := strings.SplitN(got.stdout, "Signals, newest first;", 2)[1]
		if strings.Contains(table, occurrences[0].Time.Format("2006-01-02 15:04")) ||
			!strings.Contains(table, occurrences[3].Time.Format("2006-01-02 15:04")) ||
			!strings.Contains(table, occurrences[len(occurrences)-1].Time.Format("2006-01-02 15:04")) ||
			!strings.Contains(table, "… 3 more (use --json)") {
			t.Fatalf("signal limit: %s", table)
		}
		if indicators {
			var output strings.Builder
			renderSignal(&output, backtest, *backtest.Signal, true)
			// Zero is not an unknown value; the genuinely absent high count stays empty.
			row := strings.Split(output.String(), occurrences[3].Time.Format("2006-01-02 15:04"))[1]
			row = strings.SplitN(row, "\n", 2)[0]
			if !strings.Contains(row, " 0 ") {
				t.Fatalf("zero output lost: %q", row)
			}
		}
	}
	got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT", "--json")
	var full apiclient.StrategyBacktest
	if got.code != 0 || json.Unmarshal([]byte(got.stdout), &full) != nil || full.Signal == nil || len(full.Signal.Occurrences) != len(occurrences) {
		t.Fatalf("JSON must retain all signals: %+v", got)
	}
}
