package main

import (
	"crypto-scanner/internal/apiclient"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"io"
)

// renderSignalSummary prints how often the counted signals succeeded beside
// every candle, or, for a simple signal, which is never judged, how often it
// fired.
func renderSignalSummary(w io.Writer, signal apiclient.BacktestSignal) {
	t := newTable()
	if signal.Evaluation == nil {
		t.SetColumnConfigs([]table.ColumnConfig{{Number: 2, Align: text.AlignRight}})
		t.AppendRows([]table.Row{{"Signals", len(signal.Occurrences)}, {"Candles evaluated", signal.Candles}})
		fmt.Fprintln(w, t.Render())
		return
	}
	evaluation := *signal.Evaluation
	t.AppendHeader(table.Row{"", "Count (all / counted)", "Median move to target %", "Successful", "Neutral"})
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 2, Align: text.AlignRight}, {Number: 3, Align: text.AlignRight}, {Number: 4, Align: text.AlignRight}})
	t.AppendRows([]table.Row{
		{"Signals", fmt.Sprintf("%d / %d", evaluation.Evaluated, evaluation.Signals.Count), share(evaluation.Signals.MedianMove), successful(evaluation.Signals), evaluation.Signals.Neutrals},
		{"All candles", evaluation.All.Count, share(evaluation.All.MedianMove), successful(evaluation.All), evaluation.All.Neutrals},
	})
	fmt.Fprintln(w, t.Render())
}

func renderBacktestSummary(w io.Writer, backtest apiclient.StrategyBacktest, period bool) {
	fmt.Fprintf(w, "\n%s (%s, %s)\nExecution time: %.2f s\n", backtest.Symbol, backtest.Direction, backtest.Interval, backtest.DurationMs/1000)
	if backtest.From == nil || backtest.To == nil {
		if period {
			fmt.Fprintln(w, "No stored candles in the requested period.")
		} else {
			fmt.Fprintln(w, "No evaluated candles yet.")
		}
		return
	}
	fmt.Fprintf(w, "Evaluated period: %s → %s\n", backtest.From.UTC().Format("2006-01-02 15:04 UTC"), backtest.To.UTC().Format("2006-01-02 15:04 UTC"))
	if backtest.Signal != nil {
		renderSignalSummary(w, *backtest.Signal)
		return
	}
	open := 0
	for _, trade := range backtest.Trades {
		if trade.Open {
			open++
		}
	}
	renderSummary(w, backtest.Summary, backtest.Baselines, backtest.Direction == apiclient.Short,
		table.Row{"Open trades", open, "-"}, table.Row{"Skipped signals", backtest.SkippedAlerts, "-"})
}
