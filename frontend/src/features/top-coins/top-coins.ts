import type { CriterionRequest } from "@/api/generated/models";
import { applicationConfig } from "@/config";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";

export function buildTopCoinsScanCriteria(
	settings: VolatilitySettings,
): MarketScanCriteria {
	return {
		...settings,
		hourlyMinimumRangePercent: 0,
		minimumMarketCapMillions: 0,
		minimumRangePercent: 0,
	};
}

export function buildTopCoinsCriteria(
	settings: VolatilitySettings,
): CriterionRequest[] {
	return criterionSelections(buildTopCoinsScanCriteria(settings));
}

const marketCapSort = { direction: "desc", field: "market_cap_usd" } as const;

export const topCoinsRequestOptions = {
	limit: applicationConfig.topMarketCap.resultLimit,
	sort: marketCapSort,
} as const;

// Pages ranking coins by market cap: Top selects the largest Spot coins,
// COIN-M every coin with a Binance COIN-M perpetual contract.
export const topCoinsPages = {
	"coin-m": {
		request: { sort: marketCapSort, universe: "coinm_perpetual" },
		title: "COIN-M",
	},
	"top-coins": { request: topCoinsRequestOptions, title: "Top Market Cap" },
} as const;
