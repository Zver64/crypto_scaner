package strategy

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// maxTextLength keeps messages below Telegram's 4096-character limit.
const maxTextLength = 4000

// summaryText announces an enabled or changed strategy: its kind, interval,
// coins, and market cap range, and the instruments within it whose entry is
// true now, which buy only once it turns true again.
func summaryText(entry Entry, matching []Instrument, current snapshot) string {
	kind := "strategy"
	if entry.Signal {
		kind = "signal"
	}
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
	text := "🎯 " + entry.Name + " is active\n" + capitalized(string(entry.Direction)) + " " + kind + " · " + string(entry.Interval) + " · " + coins + "\n"
	if bounds := entry.MarketCap.String(); bounds != "" {
		text += "Market cap: " + bounds + "\n"
	}
	if !entry.Signal && !entry.Exits() {
		text += "No exit: every entry signal buys.\n"
	}
	if len(matching) == 0 {
		return text + "No coin's entry is true now."
	}
	symbols := make([]string, len(matching))
	for i, instrument := range matching {
		symbols[i] = instrument.Symbol
	}
	slices.Sort(symbols)
	switch {
	case entry.Signal:
		text += "Entry true now, signaling once it turns true again: "
	case entry.Direction == DirectionShort:
		text += "Entry true now, shorting once it turns true again: "
	default:
		text += "Entry true now, buying once it turns true again: "
	}
	for i, symbol := range symbols {
		more := fmt.Sprintf(" and %d more", len(symbols)-i)
		if len(text)+len(symbol)+2+len(more) > maxTextLength {
			return strings.TrimSuffix(text, ", ") + more
		}
		text += symbol + ", "
	}
	return strings.TrimSuffix(text, ", ")
}

func capitalized(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
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
func alertText(entry Entry, alerts []alert, current snapshot) string {
	var titles []string
	lines := map[string][]string{}
	for _, item := range alerts {
		title := alertTitle(entry, item.event)
		if _, ok := lines[title]; !ok {
			titles = append(titles, title)
		}
		lines[title] = append(lines[title], alertLine(entry, item, current))
	}
	blocks := make([]string, len(titles))
	for i, title := range titles {
		block := []string{title}
		if entry.Message != "" {
			block = append(block, entry.Message)
		}
		blocks[i] = strings.Join(append(block, lines[title]...), "\n")
	}
	text := strings.Join(blocks, "\n\n")
	const tooLong = "\n…message too long"
	if len(text) <= maxTextLength {
		return text
	}
	cut := text[:maxTextLength-len(tooLong)]
	return cut[:max(0, strings.LastIndex(cut, "\n"))] + tooLong
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
	return mark + " " + entry.Name + " · " + string(entry.Interval) + " · " + kind
}

// alertLine describes the event on the instrument of item: the close of a
// signal; the price, the buy number past the first, the take profit, and the
// stop loss of a buy or short; or the price, what closed it, the return, the
// entry price, and the buys past one of a sell or cover. Then come the values
// the rule that caused it read, named as it writes them; a take profit or
// stop loss has no rule.
func alertLine(entry Entry, item alert, current snapshot) string {
	event := item.event
	parts := []string{item.instrument.Symbol + " " + formatNumber(event.Close)}
	rule := entry.Compiled
	var positions map[string]float64
	switch event.Kind {
	case TradeSignal:
	case TradeSell:
		price, reason := event.Close, "exit rule"
		switch event.Reason {
		case ExitTakeProfit:
			price, reason, rule = event.Price, "take profit", nil
		case ExitStopLoss:
			price, reason, rule = event.Price, "stop loss", nil
		default:
			rule = entry.Exit
			positions = entry.positions(event.Trade, entry.Interval.LastClosedOpenTime(current.now), event.Close)
		}
		parts = []string{item.instrument.Symbol + " at " + formatNumber(price) + " by " + reason, fmt.Sprintf("%+.2f%%", 100*event.Return), "entry " + formatNumber(event.Trade.EntryPrice())}
		if event.Trade.Buys > 1 {
			parts = append(parts, strconv.Itoa(event.Trade.Buys)+" buys")
		}
	default:
		parts[0] = item.instrument.Symbol + " at " + formatNumber(event.Close)
		if event.Buy > 1 {
			parts[0] = item.instrument.Symbol + " #" + strconv.Itoa(event.Buy) + " at " + formatNumber(event.Close)
		}
		if event.Trade.TakeProfit > 0 {
			parts = append(parts, "TP "+formatNumber(event.Trade.TakeProfit))
		}
		if event.Trade.StopLoss > 0 {
			parts = append(parts, "SL "+formatNumber(event.Trade.StopLoss))
		}
	}
	if rule != nil {
		values := rule.Values(current.resolver(entry, item.instrument.ID, positions))
		for _, label := range slices.Sorted(maps.Keys(values)) {
			parts = append(parts, label+" "+formatNumber(values[label]))
		}
	}
	return strings.Join(parts, " · ")
}

// signalMarks start the alerts of signals by the move they expect.
var signalMarks = map[Direction]string{DirectionLong: "📈", DirectionShort: "📉", DirectionSideways: "↔️"}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', 6, 64)
}
