import type { QueryClient, QueryMeta } from "@tanstack/react-query";

import {
	type analyzeMarketResponseSuccess,
	getAnalyzeMarketQueryOptions,
} from "@/api/generated/api";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { marketScanErrorNotification } from "@/features/market-scan/market-scan-screen/constants";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";

const selectAnalysis = (response: analyzeMarketResponseSuccess) =>
	response.data;

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

// Results stay cached until invalidated (for example after the administrator
// changes indicators); an invalidated scan refetches when a screen mounts it.
// Only the screen's own failed first load notifies; scans it runs report their
// own failures, and a failed refetch keeps the shown result.
export function marketScanObserverOptions(criteria: MarketScanCriteria) {
	return {
		...validatedMarketScanQueryOptions(criteria),
		gcTime: Number.POSITIVE_INFINITY,
		meta: {
			errorNotification: {
				...marketScanErrorNotification,
				initialLoadOnly: true,
			},
		} satisfies QueryMeta,
		refetchOnReconnect: false as const,
		refetchOnWindowFocus: false as const,
		retry: false as const,
		staleTime: Number.POSITIVE_INFINITY,
		select: selectAnalysis,
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
