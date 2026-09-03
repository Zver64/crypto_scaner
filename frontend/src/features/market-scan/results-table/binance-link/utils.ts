const binanceSpotQuoteAsset = "USDT";

export function binanceSpotUrl(symbol: string): string | undefined {
	const normalizedSymbol = symbol.trim().toUpperCase();
	if (
		!normalizedSymbol.endsWith(binanceSpotQuoteAsset) ||
		normalizedSymbol.length === binanceSpotQuoteAsset.length
	) {
		return undefined;
	}

	const baseAsset = normalizedSymbol.slice(0, -binanceSpotQuoteAsset.length);
	return `https://www.binance.com/en/trade/${encodeURIComponent(baseAsset)}_${binanceSpotQuoteAsset}?type=spot`;
}
