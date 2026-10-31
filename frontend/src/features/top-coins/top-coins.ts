import type { CriterionRequest } from "@/api/generated/models";
import { applicationConfig } from "@/config";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import {
	criterionSelections,
	unfilteredScanCriteria,
} from "@/features/market-scan/pipeline";

export function buildTopCoinsCriteria(
	settings: VolatilitySettings,
): CriterionRequest[] {
	return criterionSelections(unfilteredScanCriteria(settings));
}

const marketCapSort = { direction: "desc", field: "market_cap_usd" } as const;

export const topCoinsRequestOptions = {
	limit: applicationConfig.topMarketCap.resultLimit,
	sort: marketCapSort,
} as const;

// Pages ranking coins by market cap: Top selects the largest Spot coins,
// COIN-M every coin with a Binance COIN-M perpetual contract, so it does not
// highlight them.
export const topCoinsPages = {
	"coin-m": {
		highlightCoinM: false,
		request: { sort: marketCapSort, universe: "coinm_perpetual" },
		title: "COIN-M",
	},
	"top-coins": {
		highlightCoinM: true,
		request: topCoinsRequestOptions,
		title: "Top Market Cap",
	},
} as const;
