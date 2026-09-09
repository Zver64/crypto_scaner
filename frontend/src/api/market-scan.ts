import { queryOptions, useQuery } from "@tanstack/react-query";
import { hasExpectedMarketScanEvaluations } from "@/api/analysis-contract";
import {
	ApiError,
	type CriterionSelection,
	fetchMarketScan,
	type MarketScanSortOption,
} from "@/api/client";
import { getTelegramInitData } from "@/app/telegram";

interface MarketScanRequestOptions {
	limit: number;
	sort: MarketScanSortOption;
}

export function marketScanQueryOptions(
	criteria: readonly CriterionSelection[] | undefined,
	requestOptions?: MarketScanRequestOptions,
) {
	return queryOptions({
		queryFn: async () => {
			if (!criteria) throw new ApiError("unexpected_error");

			const result = await fetchMarketScan(criteria, {
				initData: getTelegramInitData(),
				...requestOptions,
			});
			if (!hasExpectedMarketScanEvaluations(result.items, criteria)) {
				throw new ApiError("unexpected_error");
			}
			return result;
		},
		queryKey: criteria
			? requestOptions
				? (["market-scan", criteria, requestOptions] as const)
				: (["market-scan", criteria] as const)
			: (["market-scan", "uncommitted"] as const),
		retry: false,
		gcTime: Number.POSITIVE_INFINITY,
		staleTime: Number.POSITIVE_INFINITY,
	});
}

export function useMarketScanQuery(
	criteria: readonly CriterionSelection[] | undefined,
	enabled: boolean,
	requestOptions?: MarketScanRequestOptions,
) {
	return useQuery({
		...marketScanQueryOptions(criteria, requestOptions),
		enabled: criteria !== undefined && enabled,
	});
}
