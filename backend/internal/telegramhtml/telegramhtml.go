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
// has none, on one line.
func Symbol(symbol string) string {
	text := OneLine(symbol)
	if url, ok := market.BinanceSpotURL(symbol); ok {
		return `<a href="` + html.EscapeString(url) + `">` + text + "</a>"
	}
	return text
}

// Title writes text as a bold message title, escaped and on one line.
func Title(text string) string {
	return "<b>" + OneLine(text) + "</b>"
}

// lineBreaks are replaced by spaces in text that must stay on one line.
var lineBreaks = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

// OneLine escapes text and joins its lines, so Truncate never cuts it.
func OneLine(text string) string {
	return html.EscapeString(lineBreaks.Replace(text))
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
