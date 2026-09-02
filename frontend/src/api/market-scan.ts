import { queryOptions, useQuery } from "@tanstack/react-query";
import { hasExpectedMarketScanEvaluations } from "@/api/analysis-contract";
import {
	ApiError,
	type CriterionSelection,
	fetchMarketScan,
} from "@/api/client";
import { getTelegramInitData } from "@/app/telegram";

export function marketScanQueryOptions(
	criteria: readonly CriterionSelection[] | undefined,
) {
	return queryOptions({
		queryFn: async () => {
			if (!criteria) throw new ApiError("unexpected_error");

			const result = await fetchMarketScan(criteria, {
				initData: getTelegramInitData(),
			});
			if (!hasExpectedMarketScanEvaluations(result.items, criteria)) {
				throw new ApiError("unexpected_error");
			}
			return result;
		},
		queryKey: criteria
			? (["market-scan", criteria] as const)
			: (["market-scan", "uncommitted"] as const),
		retry: false,
		gcTime: Number.POSITIVE_INFINITY,
		staleTime: Number.POSITIVE_INFINITY,
	});
}

export function useMarketScanQuery(
	criteria: readonly CriterionSelection[] | undefined,
	enabled: boolean,
) {
	return useQuery({
		...marketScanQueryOptions(criteria),
		enabled: criteria !== undefined && enabled,
	});
}
