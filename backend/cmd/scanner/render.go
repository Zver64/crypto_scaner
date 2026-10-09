package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/apiclient"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

// renderValidation prints a valid expression's validation; validate returns
// the errors of an invalid one.
func renderValidation(w io.Writer, validation apiclient.StrategyValidation) {
	if len(validation.MissingIndicators) == 0 {
		fmt.Fprintln(w, "ok")
		return
	}
	fmt.Fprintf(w, "ok; reads indicators that are not configured: %s\n", missingTitles(validation.MissingIndicators))
}

func missingTitles(missing []apiclient.StrategyMissingIndicator) string {
	titles := make([]string, len(missing))
	for i, indicator := range missing {
		titles[i] = indicator.Title
	}
	return strings.Join(titles, ", ")
}

// newTable returns a table in the style of every CLI table.
func newTable() table.Writer {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	return t
}

// terminalWidth is the width of the terminal w writes to, 0 when w is not
// a terminal.
func terminalWidth(w io.Writer) int {
	file, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return 0
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 0
	}
	return width
}

// renderStrategies prints the strategies of k; on a terminal, the Name, Entry,
// and Exit cells wrap to fit its width. Strategies show the direction they
// trade, signals the move they expect instead of how they trade.
func renderStrategies(w io.Writer, k kind, strategies []apiclient.Strategy) {
	renderStrategiesWidth(w, k, strategies, terminalWidth(w))
}

// renderStrategiesWidth also accepts an explicit width for non-terminal tests.
func renderStrategiesWidth(w io.Writer, k kind, strategies []apiclient.Strategy, width int) {
	header := table.Row{"ID", "State", "Name", "Direction", "Entry", "Exit", "Buys", "Market cap"}
	if k.signals {
		header = table.Row{"ID", "State", "Name", "Entry", "Signal", "Market cap"}
	}
	var rows []table.Row
	widths := make([]int, len(header))
	measure := func(row table.Row) {
		for i, value := range row {
			for line := range strings.Lines(fmt.Sprint(value)) {
				widths[i] = max(widths[i], uniseg.StringWidth(strings.TrimSuffix(line, "\n")))
			}
		}
	}
	measure(header)
	for _, strategy := range strategies {
		state := "off"
		if strategy.Enabled {
			state = "on"
		}
		if !strategy.Valid {
			state += ", invalid"
		}
		name := strategy.Name
		if strategy.Problem != nil && *strategy.Problem != "" {
			name += "\nproblem: " + *strategy.Problem
		}
		if len(strategy.MissingIndicators) > 0 {
			name += "\nnot configured: " + missingTitles(strategy.MissingIndicators)
		}
		if strategy.Message != "" {
			name += "\nmessage: " + strategy.Message
		}
		expression := strings.Join(strings.Fields(strategy.Expression), " ")
		row := table.Row{strategy.Id, state, name, string(strategy.Direction), expression, exits(strategy), buys(strategy), marketCapRange(strategy)}
		if k.signals {
			row = table.Row{strategy.Id, state, name, expression, string(strategy.Direction), marketCapRange(strategy)}
		}
		rows = append(rows, row)
		measure(row)
	}
	if width > 0 {
		// Each column has two padding cells and a border; the last border
		// adds one. Keep at least two cells per column for wide Unicode
		// characters; below that minimum, use labelled records.
		remaining := width - (3*len(header) + 1)
		if remaining < 2*len(header) {
			for _, row := range rows {
				for i, value := range row {
					fmt.Fprintln(w, wrapCells(fmt.Sprintf("%s: %v", header[i], value), width))
				}
				fmt.Fprintln(w)
			}
			return
		}
		for total := sumWidths(widths); total > remaining; total-- {
			widest := 0
			for i := range widths {
				if widths[i] > widths[widest] {
					widest = i
				}
			}
			widths[widest]--
		}
	}
	renderStrategyRows(w, header, rows, widths)
}

// renderStrategyRows uses the same grapheme width for wrapping and padding.
// go-pretty's rune-based padding cannot align joined emoji correctly.
func renderStrategyRows(w io.Writer, header table.Row, rows []table.Row, widths []int) {
	border := func(left, joint, right string) {
		parts := make([]string, len(widths))
		for i, width := range widths {
			parts[i] = strings.Repeat("─", width+2)
		}
		fmt.Fprintln(w, left+strings.Join(parts, joint)+right)
	}
	row := func(values table.Row, heading bool) {
		lines := make([][]string, len(values))
		height := 1
		for i, value := range values {
			cell := fmt.Sprint(value)
			if heading {
				cell = strings.ToUpper(cell)
			}
			lines[i] = strings.Split(wrapCells(cell, widths[i]), "\n")
			height = max(height, len(lines[i]))
		}
		for line := range height {
			fmt.Fprint(w, "│")
			for i := range values {
				cell := ""
				if line < len(lines[i]) {
					cell = lines[i][line]
				}
				padding := strings.Repeat(" ", max(0, widths[i]-uniseg.StringWidth(cell)))
				if i == 0 && !heading {
					fmt.Fprintf(w, " %s%s │", padding, cell)
				} else {
					fmt.Fprintf(w, " %s%s │", cell, padding)
				}
			}
			fmt.Fprintln(w)
		}
	}
	border("┌", "┬", "┐")
	row(header, true)
	border("├", "┼", "┤")
	for _, values := range rows {
		row(values, false)
	}
	border("└", "┴", "┘")
}

// wrapCells checks the next grapheme's display width before placing it,
// unlike WrapHard, which can overflow on mixed narrow and wide characters.
func wrapCells(value string, width int) string {
	if width <= 0 {
		return value
	}
	var out strings.Builder
	column := 0
	graphemes := uniseg.NewGraphemes(value)
	for graphemes.Next() {
		cluster := graphemes.Str()
		if strings.Contains(cluster, "\n") {
			out.WriteString(cluster)
			column = 0
			continue
		}
		cells := graphemes.Width()
		if column > 0 && column+cells > width {
			out.WriteByte('\n')
			column = 0
		}
		out.WriteString(cluster)
		column += cells
	}
	return out.String()
}

func sumWidths(widths []int) int {
	total := 0
	for _, width := range widths {
		total += width
	}
	return total
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

// buys tells how a strategy buys: one short per trade for a short one, once
// per trade with an exit, at every signal without one.
func buys(strategy apiclient.Strategy) string {
	if strategy.Direction == apiclient.Short {
		return "one short per trade"
	}
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
	if backtest.Signal != nil {
		renderSignal(w, backtest, *backtest.Signal)
		return
	}
	// A short strategy shorts and covers where a long one buys and sells.
	words := tradeWords(backtest.Direction)
	t := newTable()
	t.AppendRows([]table.Row{
		{"Coin", backtest.Symbol},
		{"Direction", backtest.Direction},
		{"Period", fmt.Sprintf("%s → %s (%s candles)", day(*backtest.From), day(*backtest.To), backtest.Interval)},
		{"Fee", fmt.Sprintf("%g%% per %s and per %s", backtest.Fee*100, words.open, words.close)},
		{"Skipped signals", fmt.Sprintf("%d (%s nothing: a trade was open, or TP/SL on the wrong side)", backtest.SkippedAlerts, words.opened)},
	})
	fmt.Fprintln(w, t.Render())
	if len(backtest.Trades) == 0 {
		// The Mini App's empty state.
		fmt.Fprintf(w, "No trades: the strategy did not %s on this coin in the stored history.\n", words.open)
		return
	}
	renderSummary(w, backtest.Summary, backtest.Baselines, words.short)
	renderTrades(w, backtest.Interval, backtest.Trades, words)
}

// renderSignal prints the backtest of a signal: the moves after its signals
// beside those after every candle, then its newest maxTradeRows signals.
func renderSignal(w io.Writer, backtest apiclient.StrategyBacktest, signal apiclient.BacktestSignal) {
	signals := "signals"
	if len(signal.Occurrences) == 1 {
		signals = "signal"
	}
	t := newTable()
	t.AppendRows([]table.Row{
		{"Coin", backtest.Symbol},
		{"Period", fmt.Sprintf("%s → %s (%s candles)", day(*backtest.From), day(*backtest.To), backtest.Interval)},
		{"Signal", fmt.Sprintf("%s, %d %s", backtest.Direction, len(signal.Occurrences), signals)},
	})
	fmt.Fprintln(w, t.Render())
	if len(signal.Occurrences) == 0 {
		fmt.Fprintln(w, "No signals: the entry did not turn true on this coin in the stored history.")
		return
	}
	fmt.Fprintln(w, "Moves from the signal candle's close over the next candles, medians: rise to the highest high, fall to the lowest low, and the range between them. Hits: "+signalHits[backtest.Direction]+". All: the same after every evaluated candle.")
	t = newTable()
	t.AppendHeader(table.Row{"Candles", "After", "Count", "Rise %", "Fall %", "Range %", "Hits %"})
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 3, Align: text.AlignRight}, {Number: 4, Align: text.AlignRight}, {Number: 5, Align: text.AlignRight}, {Number: 6, Align: text.AlignRight}, {Number: 7, Align: text.AlignRight}})
	for index, window := range signal.Windows {
		if index > 0 {
			t.AppendSeparator()
		}
		for _, row := range []struct {
			label string
			stats apiclient.BacktestSignalStats
		}{{"signals", window.Signals}, {"all", window.All}} {
			candles := ""
			if row.label == "signals" {
				candles = strconv.Itoa(window.Candles)
			}
			t.AppendRow(table.Row{candles, row.label, row.stats.Count, percent(row.stats.Rise), percent(row.stats.Fall), percent(row.stats.Range), share(row.stats.Hits)})
		}
	}
	fmt.Fprintln(w, t.Render())
	layout := candleTimeLayout(backtest.Interval)
	fmt.Fprintln(w, "Signals, newest first, with the change of the close N candles later; --json adds the values the entry read.")
	t = newTable()
	t.SetAutoIndex(true)
	header := table.Row{"Candle", "Close"}
	configs := []table.ColumnConfig{{Number: 2, Align: text.AlignRight}}
	for index, window := range signal.Windows {
		header = append(header, fmt.Sprintf("+%d %%", window.Candles))
		configs = append(configs, table.ColumnConfig{Number: 3 + index, Align: text.AlignRight})
	}
	t.AppendHeader(header)
	t.SetColumnConfigs(configs)
	occurrences := signal.Occurrences
	for _, occurrence := range slices.Backward(occurrences[max(0, len(occurrences)-maxTradeRows):]) {
		row := table.Row{occurrence.Time.UTC().Format(layout), price(occurrence.Close)}
		for _, change := range occurrence.Changes {
			row = append(row, percent(change.Change))
		}
		t.AppendRow(row)
	}
	fmt.Fprintln(w, t.Render())
	if more := len(occurrences) - maxTradeRows; more > 0 {
		fmt.Fprintf(w, "… %d more (use --json)\n", more)
	}
}

// signalHits tells which moves a signal of each direction expected.
var signalHits = map[apiclient.Direction]string{
	apiclient.Long:     "rise above the fall",
	apiclient.Short:    "fall deeper than the rise",
	apiclient.Sideways: "range below the median range of all candles",
}

// tradeWording names what a strategy does when it opens and closes a trade.
// A short strategy holds one short per trade and has no baselines.
type tradeWording struct {
	short                              bool
	open, opened, close, opens, closes string
}

// tradeWords name what a strategy trading in direction does.
func tradeWords(direction apiclient.Direction) tradeWording {
	if direction == apiclient.Short {
		return tradeWording{short: true, open: "short", opened: "shorted", close: "cover", opens: "shorts", closes: "covers"}
	}
	return tradeWording{open: "buy", opened: "bought", close: "sell", opens: "buys", closes: "sells"}
}

// renderSummary lists the Mini App's metric cards: the net profit with the
// baselines it is compared with, none for a short strategy, then the
// statistics of the closed trades.
func renderSummary(w io.Writer, summary apiclient.BacktestSummary, baselines apiclient.BacktestBaselines, short bool) {
	t := newTable()
	t.AppendHeader(table.Row{"Metric", "Strategy", "Compared with"})
	t.SetColumnConfigs([]table.ColumnConfig{{Name: "Strategy", Align: text.AlignRight}})
	stats := summary.Stats
	compared := "-"
	if !short {
		compared = "Buy & Hold " + percent(baselines.BuyAndHold) + ", DCA " + percent(baselines.Dca)
	}
	t.AppendRows([]table.Row{
		{"Net profit %", percent(&summary.NetProfit), compared},
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
// A short strategy's trades hold one short each, so they have no Buys
// column.
func renderTrades(w io.Writer, interval apiclient.CandleInterval, trades []apiclient.BacktestTrade, words tradeWording) {
	layout := candleTimeLayout(interval)
	// A title would wrap mid-sentence on the narrow tables of daily and
	// coarser intervals.
	fmt.Fprintf(w, "Trades: %s and exit rule %s fill at the open after their signal, TP and SL on the candle reaching them; an open trade is valued at the last close; returns are after fees.\n", words.opens, words.closes)
	t := newTable()
	t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Entry", "Avg price", "Buys", "Exit", "Price", "By", "Net %"})
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 2, Align: text.AlignRight}, {Number: 3, Align: text.AlignRight, Hidden: words.short}, {Number: 5, Align: text.AlignRight}, {Number: 7, Align: text.AlignRight}})
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

// candleTimeLayout writes candle times of interval: with the hour for hourly
// candles, as dates otherwise.
func candleTimeLayout(interval apiclient.CandleInterval) string {
	if interval == apiclient.N1h {
		return "2006-01-02 15:04"
	}
	return time.DateOnly
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
