import { queryOptions, useMutation, useQuery } from "@tanstack/react-query";
import { ApiError, type CriterionSelection } from "@/api/client";
import {
	type MarketScanRequestOptions,
	runMarketScan,
} from "@/api/market-scan/utils";

export function marketScanQueryOptions(
	criteria: readonly CriterionSelection[] | undefined,
	requestOptions?: MarketScanRequestOptions,
) {
	return queryOptions({
		queryFn: async () => {
			if (!criteria) throw new ApiError("unexpected_error");
			return runMarketScan(criteria, requestOptions);
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

export function useMarketScanMutation(
	requestOptions?: MarketScanRequestOptions,
) {
	return useMutation({
		mutationFn: (criteria: readonly CriterionSelection[]) =>
			runMarketScan(criteria, requestOptions),
	});
}
