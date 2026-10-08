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
	t.AppendHeader(table.Row{"ID", "State", "Name", "Entry", "Exit", "Buys"})
	for _, strategy := range strategies {
		state := "off"
		if strategy.Enabled {
			state = "on"
		}
		if !strategy.Valid {
			state = "invalid"
		}
		t.AppendRow(table.Row{strategy.Id, state, strategy.Name, strings.Join(strings.Fields(strategy.Expression), " "), exits(strategy), buys(strategy)})
	}
	fmt.Fprintln(w, t.Render())
}

// exits lists the exit rule, take profit, and stop loss of a strategy, one
// per line.
func exits(strategy apiclient.Strategy) string {
	var lines []string
	for _, exit := range []struct{ label, expression string }{
		{"", strategy.ExitExpression}, {"TP ", strategy.TakeProfitExpression}, {"SL ", strategy.StopLossExpression},
	} {
		if exit.expression != "" {
			lines = append(lines, exit.label+strings.Join(strings.Fields(exit.expression), " "))
		}
	}
	if len(lines) == 0 {
		return "-"
	}
	return strings.Join(lines, "\n")
}

// buys tells how a strategy buys: once per trade with an exit, at every
// signal without one.
func buys(strategy apiclient.Strategy) string {
	if strategy.ExitExpression != "" || strategy.TakeProfitExpression != "" || strategy.StopLossExpression != "" {
		return "one per trade"
	}
	return "every signal"
}

// maxTradeRows caps the trade list; --json prints every trade.
const maxTradeRows = 50

// renderBacktest prints the backtest; period tells that --from or --to
// limited it.
func renderBacktest(w io.Writer, backtest apiclient.StrategyBacktest, period bool) {
	if backtest.From == nil || backtest.To == nil {
		if period {
			fmt.Fprintf(w, "%s: no stored candles of this interval in the requested period.\n", backtest.Symbol)
			return
		}
		// The Mini App's empty state.
		fmt.Fprintf(w, "%s: no stored candles of this interval yet; synchronization fills them first.\n", backtest.Symbol)
		return
	}
	t := newTable()
	t.AppendRows([]table.Row{
		{"Coin", backtest.Symbol},
		{"Period", fmt.Sprintf("%s → %s (%s candles)", day(*backtest.From), day(*backtest.To), backtest.Interval)},
		{"Fee", fmt.Sprintf("%g%% per buy and per sell", backtest.Fee*100)},
		{"Skipped signals", fmt.Sprintf("%d (bought nothing: a trade was open, or TP/SL on the wrong side)", backtest.SkippedAlerts)},
	})
	fmt.Fprintln(w, t.Render())
	if len(backtest.Trades) == 0 {
		// The Mini App's empty state.
		fmt.Fprintln(w, "No trades: the strategy did not buy on this coin in the stored history.")
		return
	}
	renderSummary(w, backtest.Summary, backtest.Baselines)
	renderTrades(w, backtest.Interval, backtest.Trades)
}

// renderSummary lists the Mini App's metric cards: the net profit with the
// baselines it is compared with, then the statistics of the closed trades.
func renderSummary(w io.Writer, summary apiclient.BacktestSummary, baselines apiclient.BacktestBaselines) {
	t := newTable()
	t.AppendHeader(table.Row{"Metric", "Strategy", "Compared with"})
	t.SetColumnConfigs([]table.ColumnConfig{{Name: "Strategy", Align: text.AlignRight}})
	stats := summary.Stats
	t.AppendRows([]table.Row{
		{"Net profit %", percent(&summary.NetProfit), "Buy & Hold " + percent(baselines.BuyAndHold) + ", DCA " + percent(baselines.Dca)},
		{"Max drawdown %", fmt.Sprintf("%.2f", summary.MaxDrawdown*100), "-"},
		{"Closed trades", stats.TradeCount, "-"},
		{"Win rate %", share(stats.WinRate), "-"},
		{"Profit factor", factor(stats.ProfitFactor), "-"},
		{"Avg trade %", percent(stats.AverageTrade), "-"},
		{"Avg win %", percent(stats.AverageWin), "-"},
		{"Avg loss %", percent(stats.AverageLoss), "-"},
		{"Avg candles held", factor(stats.AverageBars), "-"},
		{"Exits TP / SL / rule", fmt.Sprintf("%d / %d / %d", stats.TakeProfitExits, stats.StopLossExits, stats.ExitRuleExits), "-"},
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
	fmt.Fprintln(w, "Trades: buys and exit rule sells fill at the open after their signal, TP and SL on the candle reaching them; an open trade is valued at the last close; returns are after fees.")
	t := newTable()
	t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Entry", "Avg price", "Buys", "Exit", "Price", "By", "Net %"})
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 2, Align: text.AlignRight}, {Number: 3, Align: text.AlignRight}, {Number: 5, Align: text.AlignRight}, {Number: 7, Align: text.AlignRight}})
	reasons := map[apiclient.BacktestTradeExitReason]string{
		apiclient.BacktestTradeExitReasonTakeProfit: "TP", apiclient.BacktestTradeExitReasonStopLoss: "SL", apiclient.BacktestTradeExitReasonExit: "rule",
	}
	for _, trade := range slices.Backward(trades[max(0, len(trades)-maxTradeRows):]) {
		exit, by := trade.ExitTime.UTC().Format(layout), "-"
		if trade.Open {
			exit = "open"
		}
		if trade.ExitReason != nil {
			by = reasons[*trade.ExitReason]
		}
		t.AppendRow(table.Row{
			trade.EntryTime.UTC().Format(layout), averagePrice(trade.EntryPrice), trade.Buys,
			exit, averagePrice(trade.ExitPrice), by, percent(&trade.NetReturn),
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

// averagePrice formats a calculated price, such as an average or a take
// profit, with 8 significant digits.
func averagePrice(value float64) string {
	rounded, _ := strconv.ParseFloat(strconv.FormatFloat(value, 'g', 8, 64), 64)
	return price(rounded)
}

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
