import type { CriterionSelection, MarketScanItem } from "@/api/client";
import {
	criterionSelections,
	defaultMarketScanCriteria,
} from "@/features/market-scan/pipeline";
import {
	type MarketScanRow,
	toMarketScanRows,
} from "@/features/market-scan/results-table/utils";
import {
	defaultMarketScanSort,
	sortMarketScanRows,
} from "@/features/market-scan/sort";

export const topCoinsCriteria: readonly CriterionSelection[] =
	criterionSelections({
		...defaultMarketScanCriteria,
		hourlyMinimumRangePercent: 0,
		minimumMarketCapMillions: 0,
		minimumRangePercent: 0,
	});

export function toTopCoinRows(
	items: readonly MarketScanItem[],
): MarketScanRow[] {
	const rowsWithMarketCap = toMarketScanRows(items).filter(
		(row) => row.marketCapUsd !== null,
	);
	return sortMarketScanRows(rowsWithMarketCap, defaultMarketScanSort).slice(
		0,
		5,
	);
}
