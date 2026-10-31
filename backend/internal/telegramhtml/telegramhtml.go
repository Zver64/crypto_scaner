// Package telegramhtml writes the parts of Telegram HTML messages that
// several senders share.
package telegramhtml

import (
	"html"
	"net/url"
	"strings"

	"crypto-scanner/internal/market"
)

// MaxLength keeps messages below Telegram's 4096-character limit. It bounds
// the bytes of the HTML, which are at least as many as the characters
// Telegram counts, so messages are cut earlier than they must be.
const MaxLength = 4000

// Coins links the coins of messages to the Main Mini App of a bot.
type Coins struct {
	appURL string
}

// NewCoins links coins to the Main Mini App of the bot named botUsername,
// which opens the coin passed as its start parameter.
func NewCoins(botUsername string) Coins {
	return Coins{appURL: "https://t.me/" + url.PathEscape(botUsername) + "?startapp="}
}

// Symbol links symbol to its page in the Mini App, followed by a link to its
// Binance Spot pair, on one line; a symbol without a Spot pair is only
// escaped.
func (coins Coins) Symbol(symbol string) string {
	text := OneLine(symbol)
	binance, ok := market.BinanceSpotURL(symbol)
	if !ok {
		return text
	}
	return link(coins.appURL+url.QueryEscape(market.NormalizeSymbol(symbol)), text) + " · " + link(binance, "Binance")
}

func link(href, text string) string {
	return `<a href="` + html.EscapeString(href) + `">` + text + "</a>"
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
