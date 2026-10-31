package strategy

import (
	"cmp"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/telegramhtml"
)

// Messages are Telegram HTML, so user text in them is escaped.

// summaryText announces an enabled or changed strategy: under its title come
// the strategy message, its coins and market cap range, and a line per
// instrument within it whose entry is true now, which buys only once it
// turns true again, the favorite ones first and starred. A message too long
// for Telegram ends at the last instrument that fits.
func summaryText(entry Entry, matching []Instrument, current snapshot, coins telegramhtml.Coins, favorite func(symbol string) bool) string {
	var scope string
	switch {
	case entry.CoinSource == CoinsAll:
		scope = "all coins"
	case entry.CoinSource == CoinsFavorites:
		scope = "favorites"
	case len(entry.Coins) == 1:
		scope = "1 coin"
	default:
		scope = strconv.Itoa(len(entry.Coins)) + " coins"
	}
	if bounds := entry.MarketCap.String(); bounds != "" {
		scope += " · market cap " + bounds
	}
	kind := string(entry.Direction) + " strategy active"
	if entry.Signal {
		kind = string(entry.Direction) + " signal active"
	}
	lines := []string{entryTitle("🎯", entry, kind)}
	if entry.Message != "" {
		lines = append(lines, html.EscapeString(entry.Message))
	}
	lines = append(lines, scope)
	if !entry.Signal && !entry.Exits() {
		lines = append(lines, "No exit: every entry signal buys.")
	}
	if len(matching) == 0 {
		return strings.Join(append(lines, "No coin's entry is true now."), "\n")
	}
	matching = slices.SortedFunc(slices.Values(matching), func(a, b Instrument) int {
		return cmp.Or(favoriteRank(favorite, a.Symbol)-favoriteRank(favorite, b.Symbol), strings.Compare(a.Symbol, b.Symbol))
	})
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
		line := star(favorite, instrument.Symbol, coins.Symbol(instrument.Symbol))
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
// "🟢 Rebound · 1h · long signal".
func entryTitle(mark string, entry Entry, kind string) string {
	return telegramhtml.Title(mark + " " + entry.Name + " · " + string(entry.Interval) + " · " + kind)
}

// alert is one event of a strategy on one instrument. past is the open time
// of the candle of a caught-up event older than the latest closed candle,
// zero for the latest one.
type alert struct {
	instrument Instrument
	event      TradeEvent
	past       time.Time
}

// alertText announces the events of one round of entry in the order of
// their candles, those of the latest candle last: under a title per kind of
// event, naming the strategy, its interval, and the kind, come the strategy
// message and a line per instrument, the favorite ones first and starred.
// Events of one candle share a title per kind, and so do those of
// consecutive candles of one kind. A message too long for Telegram ends at
// the last line that fits.
func alertText(entry Entry, alerts []alert, coins telegramhtml.Coins, favorite func(symbol string) bool) string {
	alerts = slices.Clone(alerts)
	// Events of the latest candle have no past time.
	latest := func(item alert) int {
		if item.past.IsZero() {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(alerts, func(a, b alert) int { return cmp.Or(latest(a)-latest(b), a.past.Compare(b.past)) })
	type block struct {
		title  string
		alerts []alert
	}
	var blocks []block
	for start := 0; start < len(alerts); {
		end := start + 1
		for end < len(alerts) && alerts[end].past.Equal(alerts[start].past) {
			end++
		}
		var kinds []block
		for _, item := range alerts[start:end] {
			title := alertTitle(entry, item.event)
			index := slices.IndexFunc(kinds, func(kind block) bool { return kind.title == title })
			if index < 0 {
				index = len(kinds)
				kinds = append(kinds, block{title: title})
			}
			kinds[index].alerts = append(kinds[index].alerts, item)
		}
		for _, kind := range kinds {
			if last := len(blocks) - 1; last >= 0 && blocks[last].title == kind.title {
				blocks[last].alerts = append(blocks[last].alerts, kind.alerts...)
			} else {
				blocks = append(blocks, kind)
			}
		}
		start = end
	}
	texts := make([]string, len(blocks))
	for i, item := range blocks {
		lines := []string{item.title}
		if entry.Message != "" {
			lines = append(lines, html.EscapeString(entry.Message))
		}
		slices.SortStableFunc(item.alerts, func(a, b alert) int {
			return favoriteRank(favorite, a.instrument.Symbol) - favoriteRank(favorite, b.instrument.Symbol)
		})
		for _, event := range item.alerts {
			lines = append(lines, star(favorite, event.instrument.Symbol, alertLine(coins, event)))
		}
		texts[i] = strings.Join(lines, "\n")
	}
	return telegramhtml.Truncate(strings.Join(texts, "\n\n"), "\n…message too long")
}

// favoriteRank orders the lines of favorite coins before the others.
func favoriteRank(favorite func(symbol string) bool, symbol string) int {
	if favorite(symbol) {
		return 0
	}
	return 1
}

// star marks line, about the coin of symbol, when the coin is a favorite.
func star(favorite func(symbol string) bool, symbol, line string) string {
	if favorite(symbol) {
		return "⭐ " + line
	}
	return line
}

// alertTitle names entry, its interval, and the kind of event, such as
// "🟢 Trend · 1h · buy" or "🟢 Rebound · 1h · long signal".
func alertTitle(entry Entry, event TradeEvent) string {
	mark, kind := signalMarks[entry.Direction], string(entry.Direction)+" signal"
	switch {
	case event.Kind == TradeSignal:
	case event.Kind == TradeSell && entry.Direction == DirectionShort:
		mark, kind = "🔴", "cover"
	case event.Kind == TradeSell:
		mark, kind = "🔴", "sell"
	case entry.Direction == DirectionShort:
		mark, kind = "🟢", "short"
	default:
		mark, kind = "🟢", "buy"
	}
	return entryTitle(mark, entry, kind)
}

// alertLine describes the event on the instrument of item: the close of a
// signal; the price, the buy number past the first, the take profit, and the
// stop loss of a buy or short; or the price, what closed it, the return, the
// entry price, and the buys past one of a sell or cover. The symbol links
// to its page in the Mini App and to its Binance Spot pair. A caught-up
// event of a past candle ends with the candle's open time in UTC.
func alertLine(coins telegramhtml.Coins, item alert) string {
	line := eventLine(coins, item)
	if !item.past.IsZero() {
		line += " · candle " + candleTime(item.past)
	}
	return line
}

// candleTime formats the open time of a candle, such as
// "2026-03-02 10:00 UTC".
func candleTime(at time.Time) string { return at.UTC().Format("2006-01-02 15:04 UTC") }

// eventLine is alertLine without the candle time.
func eventLine(coins telegramhtml.Coins, item alert) string {
	event := item.event
	symbol := coins.Symbol(item.instrument.Symbol)
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
var signalMarks = map[Direction]string{DirectionSimple: "📣", DirectionLong: "🟢", DirectionShort: "🔴", DirectionSideways: "↕️"}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', 6, 64)
}
