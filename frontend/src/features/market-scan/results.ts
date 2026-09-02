import type {
	MarketScanItem,
	UnresolvedInstrumentCode,
} from "../../api/client";
import { marketCapEvaluation, volatilityEvaluation } from "./criteria";
import { dailyVolatilityKey, hourlyVolatilityKey } from "./pipeline";
import type { MarketScanSort } from "./sort";

const rangePercentFormatter = new Intl.NumberFormat("en", {
	maximumSignificantDigits: 3,
});

const marketCapFormatter = new Intl.NumberFormat("en", {
	maximumFractionDigits: 1,
	notation: "compact",
});

const binanceSpotQuoteAsset = "USDT";

export function formatRangePercent(value: number): string {
	return `${rangePercentFormatter.format(value)}%`;
}

export function formatMarketCapUsd(value: number): string {
	return `$${marketCapFormatter.format(value)}`;
}

const marketCapUnavailableReasons: Record<UnresolvedInstrumentCode, string> = {
	mapping_conflict:
		"Multiple market capitalization matches were found for this instrument.",
	mapping_not_found:
		"No market capitalization match was found for this instrument.",
	mapping_provider_unavailable:
		"Market capitalization mapping data is temporarily unavailable.",
	market_cap_missing:
		"Market capitalization data is unavailable for this instrument.",
};

export function marketCapUnavailableReason(
	code: UnresolvedInstrumentCode,
): string {
	return marketCapUnavailableReasons[code];
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

export function sortMarketScanItems(
	items: readonly MarketScanItem[],
	sort: MarketScanSort,
): MarketScanItem[] {
	return [...items].sort((left, right) => {
		const leftValue = marketScanSortValue(left, sort.column);
		const rightValue = marketScanSortValue(right, sort.column);
		const valueComparison =
			sort.direction === "desc"
				? rightValue - leftValue
				: leftValue - rightValue;

		return valueComparison || left.symbol.localeCompare(right.symbol, "en-US");
	});
}

function marketScanSortValue(
	item: MarketScanItem,
	column: MarketScanSort["column"],
): number {
	if (column === "market_cap") {
		return marketCapEvaluation(item.evaluations)?.marketCapUsd ?? 0;
	}

	return (
		volatilityEvaluation(
			item.evaluations,
			column === "daily_volatility" ? dailyVolatilityKey : hourlyVolatilityKey,
		)?.rangePercent ?? 0
	);
}
