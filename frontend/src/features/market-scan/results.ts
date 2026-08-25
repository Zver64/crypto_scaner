import type { MarketScanItem } from "../../api/client";

const rangePercentFormatter = new Intl.NumberFormat("en", {
	maximumSignificantDigits: 3,
});

const binanceSpotQuoteAsset = "USDT";

export function formatRangePercent(value: number): string {
	return `${rangePercentFormatter.format(value)}%`;
}

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

export function filterMarketScanItems(
	items: readonly MarketScanItem[],
	filter: string,
): MarketScanItem[] {
	const normalizedFilter = filter.trim().toLocaleLowerCase("en-US");
	if (!normalizedFilter) {
		return [...items];
	}

	return items.filter((item) =>
		item.symbol.toLocaleLowerCase("en-US").includes(normalizedFilter),
	);
}
