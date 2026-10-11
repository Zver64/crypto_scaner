package main

import (
	"crypto-scanner/internal/apiclient"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"io"
)

func renderSignalSummary(w io.Writer, signal apiclient.BacktestSignal) {
	t := newTable()
	t.AppendHeader(table.Row{"", "Count (all / counted)", "Median move to target %", "Successful", "Neutral"})
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 2, Align: text.AlignRight}, {Number: 3, Align: text.AlignRight}, {Number: 4, Align: text.AlignRight}})
	t.AppendRows([]table.Row{
		{"Signals", fmt.Sprintf("%d / %d", signal.Evaluated, signal.Signals.Count), share(signal.Signals.MedianMove), successful(signal.Signals), signal.Signals.Neutrals},
		{"All candles", signal.All.Count, share(signal.All.MedianMove), successful(signal.All), signal.All.Neutrals},
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
