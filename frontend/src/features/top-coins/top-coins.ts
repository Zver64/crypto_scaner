import type {
	CriterionRequest,
	MarketAnalysisItem,
} from "@/api/generated/models";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";
import {
	type MarketScanRow,
	toMarketScanRows,
} from "@/features/market-scan/results-table/utils";
import type { TopCoinsSettings } from "@/features/top-coins/settings-form/types";

export function buildTopCoinsScanCriteria(
	settings: TopCoinsSettings,
): MarketScanCriteria {
	return {
		...settings,
		hourlyMinimumRangePercent: 0,
		minimumMarketCapMillions: 0,
		minimumRangePercent: 0,
	};
}

export function buildTopCoinsCriteria(
	settings: TopCoinsSettings,
): CriterionRequest[] {
	return criterionSelections(buildTopCoinsScanCriteria(settings));
}

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
