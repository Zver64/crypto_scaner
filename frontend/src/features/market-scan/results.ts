import type { MarketScanItem } from "../../api/client";

const rangePercentFormatter = new Intl.NumberFormat("en", {
	maximumSignificantDigits: 3,
});

export function formatRangePercent(value: number): string {
	return `${rangePercentFormatter.format(value)}%`;
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
