import { useQueries } from "@tanstack/react-query";

import {
	getListChartIndicatorsQueryOptions,
	type ListChartIndicatorsQueryResult,
} from "@/api/generated/api";
import type { ChartIndicatorCatalog } from "@/api/generated/models";
import { chartIntervals } from "@/features/candle-chart/intervals";
import { intervalRecord } from "@/features/candle-chart/utils";
import type { ChartCatalogs } from "@/features/instrument-analysis/live-candle-store";

// The administrator configures which indicators the charts of each interval
// show; the catalogs say which indicators the live chart subscribes to and
// which panes it has, and the backend draws them. The select and combine results are structurally
// shared, so they keep their references while the catalogs do not change.
export function useChartIndicators(enabled: boolean): {
	catalogs?: ChartCatalogs;
	failed: boolean;
} {
	return useQueries({
		queries: chartIntervals.map((interval) =>
			getListChartIndicatorsQueryOptions(
				{ interval },
				{ query: { enabled, select: selectCatalog, staleTime: 60_000 } },
			),
		),
		combine: combineCatalogs,
	});
}

// Defined at module level so select runs only when the response changes.
function selectCatalog(
	response: ListChartIndicatorsQueryResult,
): ChartIndicatorCatalog {
	return response.data;
}

// Defined at module level so the combined result keeps its reference while the
// query results do not change.
function combineCatalogs(
	results: readonly { data?: ChartIndicatorCatalog; isError: boolean }[],
): {
	catalogs?: ChartCatalogs;
	failed: boolean;
} {
	const failed = results.some((result) => result.isError);
	if (results.some((result) => result.data === undefined)) return { failed };
	return {
		catalogs: intervalRecord(
			(interval) =>
				results[chartIntervals.indexOf(interval)]?.data ?? {
					items: [],
					panes: [],
				},
		),
		failed,
	};
}
