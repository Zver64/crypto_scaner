// Package telegramhtml writes the parts of Telegram HTML messages that
// several senders share.
package telegramhtml

import (
	"html"
	"strings"

	"crypto-scanner/internal/market"
)

// MaxLength keeps messages below Telegram's 4096-character limit. It bounds
// the bytes of the HTML, which are at least as many as the characters
// Telegram counts, so messages are cut earlier than they must be.
const MaxLength = 4000

// Symbol links symbol to its Binance Spot pair, or only escapes it when it
// has none.
func Symbol(symbol string) string {
	text := html.EscapeString(symbol)
	if url, ok := market.BinanceSpotURL(symbol); ok {
		return `<a href="` + html.EscapeString(url) + `">` + text + "</a>"
	}
	return text
}

// Title writes text as a bold message title, escaped.
func Title(text string) string {
	return "<b>" + html.EscapeString(text) + "</b>"
}

// Truncate ends text, whose tags each open and close within one line, at the
// last line that fits MaxLength with marker, so no tag is cut.
func Truncate(text, marker string) string {
	if len(text) <= MaxLength {
		return text
	}
	cut := text[:MaxLength-len(marker)]
	return cut[:max(0, strings.LastIndex(cut, "\n"))] + marker
}
