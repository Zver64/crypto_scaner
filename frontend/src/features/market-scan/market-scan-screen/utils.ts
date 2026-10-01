import type { QueryClient } from "@tanstack/react-query";
import {
	type analyzeMarket,
	getAnalyzeMarketQueryOptions,
} from "@/api/generated/api";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";

function validatedMarketScanQueryOptions(criteria: MarketScanCriteria) {
	const selections = criterionSelections(criteria);
	const request = { criteria: selections };
	const generatedOptions = getAnalyzeMarketQueryOptions(request);
	const generatedQueryFn = generatedOptions.queryFn;
	if (typeof generatedQueryFn !== "function") {
		throw new Error("Analyze Market query function is unavailable");
	}

	return {
		...generatedOptions,
		queryFn: async (context: Parameters<typeof generatedQueryFn>[0]) => {
			const response = await generatedQueryFn(context);
			if (!hasExpectedMarketScanResult(response.data, selections)) {
				throw unexpectedApiError();
			}
			return response;
		},
	};
}

export function marketScanObserverOptions(criteria: MarketScanCriteria) {
	return {
		...validatedMarketScanQueryOptions(criteria),
		gcTime: Number.POSITIVE_INFINITY,
		refetchOnMount: false as const,
		refetchOnReconnect: false as const,
		refetchOnWindowFocus: false as const,
		retry: false as const,
		staleTime: Number.POSITIVE_INFINITY,
		select: (response: Awaited<ReturnType<typeof analyzeMarket>>) =>
			response.data,
	};
}

export function fetchFreshMarketScan(
	queryClient: QueryClient,
	criteria: MarketScanCriteria,
) {
	return queryClient.fetchQuery({
		...validatedMarketScanQueryOptions(criteria),
		gcTime: Number.POSITIVE_INFINITY,
		retry: false,
		staleTime: 0,
	});
}

export function marketScanMutationOptions(queryClient: QueryClient) {
	return {
		mutationFn: (criteria: MarketScanCriteria) =>
			fetchFreshMarketScan(queryClient, criteria),
		retry: false as const,
	};
}
