import type {
	CriterionRequest,
	MarketAnalysisItem,
} from "@/api/generated/models";
import {
	criterionSelections,
	defaultMarketScanCriteria,
} from "@/features/market-scan/pipeline";
import {
	type MarketScanRow,
	toMarketScanRows,
} from "@/features/market-scan/results-table/utils";

export const topCoinsScanCriteria = {
	...defaultMarketScanCriteria,
	hourlyMinimumRangePercent: 0,
	minimumMarketCapMillions: 0,
	minimumRangePercent: 0,
};

export const topCoinsCriteria: readonly CriterionRequest[] =
	criterionSelections(topCoinsScanCriteria);

export const topCoinsRequestOptions = {
	limit: 10,
	sort: {
		direction: "desc",
		field: "market_cap_usd",
	},
} as const;

export function toTopCoinRows(
	items: readonly MarketAnalysisItem[],
): MarketScanRow[] {
	return toMarketScanRows(items);
}
