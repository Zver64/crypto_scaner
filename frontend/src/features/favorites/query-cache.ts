import type { QueryClient } from "@tanstack/react-query";

import {
	getAnalyzeMarketQueryKey,
	getListFavoritesQueryKey,
} from "@/api/generated/api";
import {
	type MarketAnalysisRequest,
	MarketUniverse,
} from "@/api/generated/models";

// Whether a query analyzes the favorites universe.
function isFavoritesAnalysis({
	queryKey,
}: {
	queryKey: readonly unknown[];
}): boolean {
	const [method, path] = getAnalyzeMarketQueryKey();
	const request = queryKey[2] as MarketAnalysisRequest | undefined;
	return (
		queryKey[0] === method &&
		queryKey[1] === path &&
		request?.universe === MarketUniverse.favorites
	);
}

export function invalidateFavoriteQueries(queryClient: QueryClient) {
	void queryClient.invalidateQueries({
		queryKey: getListFavoritesQueryKey(),
	});
	void queryClient.invalidateQueries({ predicate: isFavoritesAnalysis });
}
