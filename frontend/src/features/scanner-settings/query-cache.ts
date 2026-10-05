import type { QueryClient } from "@tanstack/react-query";
import {
	getAnalyzeMarketQueryKey,
	getListChartIndicatorsQueryKey,
	getListScannerIndicatorsQueryKey,
	getListStrategiesQueryKey,
	getListStrategyVariablesQueryKey,
} from "@/api/generated/api";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";

// Indicators change charts, table columns, and strategy variables everywhere.
// The returned promise settles once the indicator list and the strategy
// variables are current again.
export function invalidateScannerIndicatorQueries(queryClient: QueryClient) {
	const list = queryClient.invalidateQueries({
		queryKey: getListScannerIndicatorsQueryKey(),
	});
	void queryClient.invalidateQueries({
		queryKey: getListChartIndicatorsQueryKey(),
	});
	const variables = queryClient.invalidateQueries({
		queryKey: getListStrategyVariablesQueryKey(),
	});
	void queryClient.invalidateQueries({
		queryKey: getListStrategiesQueryKey(),
	});
	void queryClient.invalidateQueries({
		queryKey: getAnalyzeMarketQueryKey().slice(0, 2),
	});
	invalidateFavoriteQueries(queryClient);
	return Promise.all([list, variables]);
}
