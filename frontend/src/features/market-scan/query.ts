import { useQuery } from "@tanstack/react-query";
import { ApiError, fetchMarketScan } from "@/api/client";
import { getTelegramInitData } from "@/app/telegram";
import {
	criterionSelections,
	type MarketScanCriteria,
	marketScanCriteriaIdentity,
} from "./pipeline";
import { hasRequiredMarketScanEvaluations } from "./results";

export function marketScanQueryKey(criteria: MarketScanCriteria) {
	return ["market-scan", ...marketScanCriteriaIdentity(criteria)] as const;
}

export function useMarketScanQuery(
	criteria: MarketScanCriteria | undefined,
	enabled: boolean,
) {
	return useQuery({
		enabled: criteria !== undefined && enabled,
		queryFn: async () => {
			if (!criteria) throw new ApiError("unexpected_error");

			const result = await fetchMarketScan(criterionSelections(criteria), {
				initData: getTelegramInitData(),
			});
			if (
				!hasRequiredMarketScanEvaluations(
					result.items,
					criteria.minimumMarketCapMillions > 0,
				)
			) {
				throw new ApiError("unexpected_error");
			}
			return result;
		},
		queryKey: criteria
			? marketScanQueryKey(criteria)
			: (["market-scan", "uncommitted"] as const),
		retry: false,
		gcTime: Number.POSITIVE_INFINITY,
		staleTime: Number.POSITIVE_INFINITY,
	});
}
