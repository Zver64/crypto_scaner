package alerts

import (
	"html"
	"strings"

	"crypto-scanner/internal/telegramhtml"
)

// message names the symbol, the price that hit the alert, and its target, in
// Telegram HTML.
func message(coins telegramhtml.Coins, fired Fired) string {
	return telegramhtml.Title("🔔 "+fired.Alert.Symbol+" · price alert") + "\n" +
		coins.Symbol(fired.Alert.Symbol) + " " + html.EscapeString(trimDecimal(fired.Price)) + " · target " + html.EscapeString(fired.Alert.Target)
}

// trimDecimal drops the trailing zeros Binance pads prices with, such as
// 65012.30000000 to 65012.3.
func trimDecimal(price string) string {
	if !strings.Contains(price, ".") {
		return price
	}
	trimmed := strings.TrimSuffix(strings.TrimRight(price, "0"), ".")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}
