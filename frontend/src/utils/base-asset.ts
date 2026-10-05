/** Returns the base coin of a USDT-quoted symbol, e.g. BTC for BTCUSDT. */
export function baseAssetOfUsdtSymbol(symbol: string): string {
	return symbol.endsWith("USDT") && symbol.length > 4
		? symbol.slice(0, -4)
		: symbol;
}
