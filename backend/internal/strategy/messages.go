package strategy

import (
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/telegramhtml"
)

// Messages are Telegram HTML, so user text in them is escaped.

// summaryText announces an enabled or changed strategy: under its title come
// the strategy message, its coins and market cap range, and a line per
// instrument within it whose entry is true now, which buys only once it
// turns true again. A message too long for Telegram ends at the last
// instrument that fits.
func summaryText(entry Entry, matching []Instrument, current snapshot) string {
	var coins string
	switch {
	case entry.CoinSource == CoinsAll:
		coins = "all coins"
	case entry.CoinSource == CoinsFavorites:
		coins = "favorites"
	case len(entry.Coins) == 1:
		coins = "1 coin"
	default:
		coins = strconv.Itoa(len(entry.Coins)) + " coins"
	}
	if bounds := entry.MarketCap.String(); bounds != "" {
		coins += " · market cap " + bounds
	}
	kind := string(entry.Direction) + " strategy active"
	if entry.Signal {
		kind = string(entry.Direction) + " signal active"
	}
	lines := []string{entryTitle("🎯", entry, kind)}
	if entry.Message != "" {
		lines = append(lines, html.EscapeString(entry.Message))
	}
	lines = append(lines, coins)
	if !entry.Signal && !entry.Exits() {
		lines = append(lines, "No exit: every entry signal buys.")
	}
	if len(matching) == 0 {
		return strings.Join(append(lines, "No coin's entry is true now."), "\n")
	}
	matching = slices.SortedFunc(slices.Values(matching), func(a, b Instrument) int { return strings.Compare(a.Symbol, b.Symbol) })
	switch {
	case entry.Signal:
		lines = append(lines, "Entry true now, signaling once it turns true again:")
	case entry.Direction == DirectionShort:
		lines = append(lines, "Entry true now, shorting once it turns true again:")
	default:
		lines = append(lines, "Entry true now, buying once it turns true again:")
	}
	text := strings.Join(lines, "\n")
	for i, instrument := range matching {
		line := telegramhtml.Symbol(instrument.Symbol)
		if price, ok := current.closePrice(entry, instrument.ID); ok {
			line += " " + formatNumber(price)
		}
		more := fmt.Sprintf("\n…and %d more", len(matching)-i)
		if len(text)+1+len(line)+len(more) > telegramhtml.MaxLength {
			return text + more
		}
		text += "\n" + line
	}
	return text
}

// haltText tells the administrator that err disabled entry.
func haltText(entry Entry, err error) string {
	return entryTitle("⛔", entry, "disabled") + "\n" + html.EscapeString(err.Error())
}

// entryTitle is the bold title of a message about entry, naming the
// strategy, its interval, and kind after mark, such as
// "📈 Rebound · 1h · long signal".
func entryTitle(mark string, entry Entry, kind string) string {
	return telegramhtml.Title(mark + " " + entry.Name + " · " + string(entry.Interval) + " · " + kind)
}

// alert is one event of a strategy on one instrument.
type alert struct {
	instrument Instrument
	event      TradeEvent
}

// alertText announces the events of one round of entry: under a title per
// kind of event, naming the strategy, its interval, and the kind, come the
// strategy message and a line per instrument. A message too long for
// Telegram ends at the last line that fits.
func alertText(entry Entry, alerts []alert) string {
	var titles []string
	lines := map[string][]string{}
	for _, item := range alerts {
		title := alertTitle(entry, item.event)
		if _, ok := lines[title]; !ok {
			titles = append(titles, title)
		}
		lines[title] = append(lines[title], alertLine(item))
	}
	blocks := make([]string, len(titles))
	for i, title := range titles {
		block := []string{title}
		if entry.Message != "" {
			block = append(block, html.EscapeString(entry.Message))
		}
		blocks[i] = strings.Join(append(block, lines[title]...), "\n")
	}
	return telegramhtml.Truncate(strings.Join(blocks, "\n\n"), "\n…message too long")
}

// alertTitle names entry, its interval, and the kind of event, such as
// "🟢 Trend · 1h · buy" or "📈 Rebound · 1h · long signal".
func alertTitle(entry Entry, event TradeEvent) string {
	mark, kind := signalMarks[entry.Direction], string(entry.Direction)+" signal"
	switch {
	case event.Kind == TradeSignal:
	case event.Kind == TradeSell && entry.Direction == DirectionShort:
		mark, kind = "🔺", "cover"
	case event.Kind == TradeSell:
		mark, kind = "🔴", "sell"
	case entry.Direction == DirectionShort:
		mark, kind = "🔻", "short"
	default:
		mark, kind = "🟢", "buy"
	}
	return entryTitle(mark, entry, kind)
}

// alertLine describes the event on the instrument of item: the close of a
// signal; the price, the buy number past the first, the take profit, and the
// stop loss of a buy or short; or the price, what closed it, the return, the
// entry price, and the buys past one of a sell or cover. The symbol links
// to its Binance Spot pair.
func alertLine(item alert) string {
	event := item.event
	symbol := telegramhtml.Symbol(item.instrument.Symbol)
	switch event.Kind {
	case TradeSignal:
		return symbol + " " + formatNumber(event.Close)
	case TradeSell:
		price, reason := event.Close, "exit rule"
		switch event.Reason {
		case ExitTakeProfit:
			price, reason = event.Price, "take profit"
		case ExitStopLoss:
			price, reason = event.Price, "stop loss"
		}
		parts := []string{symbol + " at " + formatNumber(price) + " by " + reason, fmt.Sprintf("%+.2f%%", 100*event.Return), "entry " + formatNumber(event.Trade.EntryPrice())}
		if event.Trade.Buys > 1 {
			parts = append(parts, strconv.Itoa(event.Trade.Buys)+" buys")
		}
		return strings.Join(parts, " · ")
	}
	if event.Buy > 1 {
		symbol += " #" + strconv.Itoa(event.Buy)
	}
	parts := []string{symbol + " at " + formatNumber(event.Close)}
	if event.Trade.TakeProfit > 0 {
		parts = append(parts, "TP "+formatNumber(event.Trade.TakeProfit))
	}
	if event.Trade.StopLoss > 0 {
		parts = append(parts, "SL "+formatNumber(event.Trade.StopLoss))
	}
	return strings.Join(parts, " · ")
}

// signalMarks start the alerts of signals by the move they expect.
var signalMarks = map[Direction]string{DirectionLong: "📈", DirectionShort: "📉", DirectionSideways: "↔️"}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', 6, 64)
}
