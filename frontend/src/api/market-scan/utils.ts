import { hasExpectedMarketScanEvaluations } from "@/api/analysis-contract";
import {
	ApiError,
	type CriterionSelection,
	fetchMarketScan,
	type MarketScanSortOption,
} from "@/api/client";
import { getTelegramInitData } from "@/app/telegram";

export interface MarketScanRequestOptions {
	limit: number;
	sort: MarketScanSortOption;
}

export async function runMarketScan(
	criteria: readonly CriterionSelection[],
	requestOptions?: MarketScanRequestOptions,
) {
	const result = await fetchMarketScan(criteria, {
		initData: getTelegramInitData(),
		...requestOptions,
	});
	if (!hasExpectedMarketScanEvaluations(result.items, criteria)) {
		throw new ApiError("unexpected_error");
	}
	return result;
}
