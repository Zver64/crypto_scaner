package market

import "strings"

// BinanceSpotQuoteAsset is the quote asset of the Binance Spot pairs.
const BinanceSpotQuoteAsset = "USDT"

// NormalizeSymbol returns the canonical exchange symbol form.
func NormalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// SpotBaseAsset returns the base asset of a Binance Spot USDT pair.
func SpotBaseAsset(symbol string) (string, bool) {
	base, ok := strings.CutSuffix(NormalizeSymbol(symbol), BinanceSpotQuoteAsset)
	return base, ok && base != ""
}

// BinanceSpotURL opens the pair of symbol on Binance Spot.
func BinanceSpotURL(symbol string) (string, bool) {
	base, ok := SpotBaseAsset(symbol)
	if !ok {
		return "", false
	}
	return "https://www.binance.com/en/trade/" + base + "_" + BinanceSpotQuoteAsset + "?type=spot", true
}
