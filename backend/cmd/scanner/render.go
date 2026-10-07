package main

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/apiclient"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

func renderValidation(w io.Writer, validation apiclient.StrategyValidation) {
	for _, problem := range validation.Errors {
		fmt.Fprintln(w, "error:", problem)
	}
	if len(validation.Errors) > 0 {
		return
	}
	if len(validation.MissingIndicators) == 0 {
		fmt.Fprintln(w, "ok")
		return
	}
	titles := make([]string, len(validation.MissingIndicators))
	for i, missing := range validation.MissingIndicators {
		titles[i] = missing.Title
	}
	fmt.Fprintf(w, "ok; indicators will be added when saving: %s\n", strings.Join(titles, ", "))
}

// newTable returns a table in the style of every CLI table.
func newTable() table.Writer {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	return t
}

func renderStrategies(w io.Writer, strategies []apiclient.Strategy) {
	t := newTable()
	t.AppendHeader(table.Row{"ID", "State", "Name", "Expression"})
	for _, strategy := range strategies {
		state := "off"
		if strategy.Enabled {
			state = "on"
		}
		if !strategy.Valid {
			state = "invalid"
		}
		t.AppendRow(table.Row{strategy.Id, state, strategy.Name, strings.Join(strings.Fields(strategy.Expression), " ")})
	}
	fmt.Fprintln(w, t.Render())
}

// maxTradeRows caps the trade list; --json prints every trade.
const maxTradeRows = 50

func renderBacktest(w io.Writer, backtest apiclient.StrategyBacktest) {
	if backtest.From == nil || backtest.To == nil {
		// The Mini App's empty state.
		fmt.Fprintf(w, "%s: no stored candles of this interval yet; synchronization fills them first.\n", backtest.Symbol)
		return
	}
	t := newTable()
	t.AppendRows([]table.Row{
		{"Coin", backtest.Symbol},
		{"Period", fmt.Sprintf("%s → %s (%s candles)", day(*backtest.From), day(*backtest.To), backtest.Interval)},
		{"Hold", fmt.Sprintf("%d candles", backtest.Hold)},
		{"Fee", fmt.Sprintf("%g%% per buy and per sell", backtest.Fee*100)},
		{"Skipped alerts", fmt.Sprintf("%d (a trade was open)", backtest.SkippedAlerts)},
		{"Unfinished", fmt.Sprintf("%d (history ended or had a gap before the exit)", backtest.UnfinishedTrades)},
	})
	fmt.Fprintln(w, t.Render())
	if len(backtest.Trades) == 0 {
		// The Mini App's empty state.
		if backtest.UnfinishedTrades > 0 {
			fmt.Fprintf(w, "No trades: every trade is still unfinished (%d): the stored history ends or has a gap before its exit.\n", backtest.UnfinishedTrades)
		} else {
			fmt.Fprintln(w, "No trades: the strategy did not alert on this coin in the stored history.")
		}
		return
	}
	renderSummary(w, backtest.Summary, backtest.Baselines)
	renderAverages(w, backtest.Summary.Stats, backtest.Baselines.EveryCandle)
	renderTrades(w, backtest.Interval, backtest.Trades)
}

// renderSummary lists the Mini App's metric cards: each strategy metric with
// the baseline it is compared with.
func renderSummary(w io.Writer, summary apiclient.BacktestSummary, baselines apiclient.BacktestBaselines) {
	t := newTable()
	t.AppendHeader(table.Row{"Metric", "Strategy", "Compared with"})
	t.SetColumnConfigs([]table.ColumnConfig{{Name: "Strategy", Align: text.AlignRight}})
	strategy, every := summary.Stats, baselines.EveryCandle
	t.AppendRows([]table.Row{
		{"Net profit %", percent(&summary.NetProfit), "Buy & Hold " + percent(baselines.BuyAndHold)},
		{"Trades", strategy.TradeCount, fmt.Sprintf("Every candle %d", every.TradeCount)},
		{"Win rate %", share(strategy.WinRate), "Every candle " + share(every.WinRate)},
		{"Profit factor", factor(strategy.ProfitFactor), "Every candle " + factor(every.ProfitFactor)},
		{"Max drawdown %", fmt.Sprintf("%.2f", summary.MaxDrawdown*100), "-"},
	})
	fmt.Fprintln(w, t.Render())
}

// renderAverages compares the average returns with every candle.
func renderAverages(w io.Writer, strategy, every apiclient.BacktestTradeStats) {
	t := newTable()
	t.AppendHeader(table.Row{"Metric", "Strategy", "Every candle"})
	t.SetColumnConfigs([]table.ColumnConfig{{Name: "Strategy", Align: text.AlignRight}, {Name: "Every candle", Align: text.AlignRight}})
	t.AppendRows([]table.Row{
		{"Avg trade %", percent(strategy.AverageTrade), percent(every.AverageTrade)},
		{"Avg win %", percent(strategy.AverageWin), percent(every.AverageWin)},
		{"Avg loss %", percent(strategy.AverageLoss), percent(every.AverageLoss)},
	})
	fmt.Fprintln(w, t.Render())
}

// renderTrades lists the newest maxTradeRows trades, newest first like the
// Mini App.
func renderTrades(w io.Writer, interval apiclient.CandleInterval, trades []apiclient.BacktestTrade) {
	layout := time.DateOnly
	if interval == apiclient.N1h {
		layout = "2006-01-02 15:04"
	}
	// A title would wrap mid-sentence on the narrow tables of daily and
	// coarser intervals.
	fmt.Fprintln(w, "Trades: buy at the open of the candle after an alert, sell at the close of the last held candle; returns are after fees.")
	t := newTable()
	t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Entry", "Price", "Exit", "Price", "Net %"})
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 2, Align: text.AlignRight}, {Number: 4, Align: text.AlignRight}, {Number: 5, Align: text.AlignRight}})
	for _, trade := range slices.Backward(trades[max(0, len(trades)-maxTradeRows):]) {
		t.AppendRow(table.Row{
			trade.EntryTime.UTC().Format(layout), price(trade.EntryPrice),
			trade.ExitTime.UTC().Format(layout), price(trade.ExitPrice), percent(&trade.NetReturn),
		})
	}
	fmt.Fprintln(w, t.Render())
	if more := len(trades) - maxTradeRows; more > 0 {
		fmt.Fprintf(w, "… %d more (use --json)\n", more)
	}
}

func factor(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *value)
}

func price(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

// percent formats a fractional return as a signed percentage.
func percent(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%+.2f", *value*100)
}

// share formats a fraction as a percentage with up to 2 decimals, like the
// Mini App.
func share(value *float64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatFloat(math.Round(*value*10000)/100, 'f', -1, 64)
}

func day(value time.Time) string { return value.UTC().Format(time.DateOnly) }
