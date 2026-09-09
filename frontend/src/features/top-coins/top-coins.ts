import type { CriterionSelection, MarketScanItem } from "@/api/client";
import {
	criterionSelections,
	defaultMarketScanCriteria,
} from "@/features/market-scan/pipeline";
import {
	type MarketScanRow,
	toMarketScanRows,
} from "@/features/market-scan/results-table/utils";

export const topCoinsCriteria: readonly CriterionSelection[] =
	criterionSelections({
		...defaultMarketScanCriteria,
		hourlyMinimumRangePercent: 0,
		minimumMarketCapMillions: 0,
		minimumRangePercent: 0,
	});

export const topCoinsRequestOptions = {
	limit: 10,
	sort: {
		direction: "desc",
		field: "market_cap_usd",
	},
} as const;

export function toTopCoinRows(
	items: readonly MarketScanItem[],
): MarketScanRow[] {
	return toMarketScanRows(items);
}
