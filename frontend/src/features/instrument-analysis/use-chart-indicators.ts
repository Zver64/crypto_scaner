import { useQueries } from "@tanstack/react-query";

import {
	getListChartIndicatorsQueryOptions,
	type ListChartIndicatorsQueryResult,
	listChartIndicators,
} from "@/api/generated/api";
import type {
	CandleInterval,
	ChartIndicatorDefinition,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";
import type { ChartIndicatorLayout } from "@/features/candle-chart";
import { createIndicatorLayout } from "@/features/candle-chart/utils";
import {
	type ChartCatalogs,
	chartIntervals,
} from "@/features/instrument-analysis/live-candle-store";

interface IntervalCatalog {
	catalog: ChartIndicatorDefinition[];
	layout: ChartIndicatorLayout;
}

// The administrator configures which indicators the charts of each interval
// show; the backend serves them with how to draw them. The catalogs drive both
// the live chart subscriptions and the rendering. Each catalog is validated
// when fetched, so an unexpected one fails its query and reaches the query
// cache error handler; its layout is created in select, and the select and combine results are structurally
// shared, so they keep their references while the catalogs do not change.
export function useChartIndicators(enabled: boolean): {
	catalogs?: ChartCatalogs;
	failed: boolean;
	indicators?: Record<CandleInterval, ChartIndicatorLayout>;
} {
	return useQueries({
		queries: chartIntervals.map((interval) =>
			getListChartIndicatorsQueryOptions(
				{ interval },
				{
					query: {
						enabled,
						queryFn: async ({ signal }) => {
							const response = await listChartIndicators(
								{ interval },
								{ signal },
							);
							validateCatalog(response.data.items);
							return response;
						},
						select: selectCatalog,
						staleTime: 60_000,
					},
				},
			),
		),
		combine: combineCatalogs,
	});
}

// Defined at module level so select runs only when the response changes.
function selectCatalog(
	response: ListChartIndicatorsQueryResult,
): IntervalCatalog {
	const catalog = response.data.items;
	return { catalog, layout: createIndicatorLayout(catalog) };
}

// Defined at module level so the combined result keeps its reference while the
// query results do not change.
function combineCatalogs(
	results: readonly { data?: IntervalCatalog; isError: boolean }[],
): {
	catalogs?: ChartCatalogs;
	failed: boolean;
	indicators?: Record<CandleInterval, ChartIndicatorLayout>;
} {
	const failed = results.some((result) => result.isError);
	const catalog = (interval: CandleInterval) =>
		results[chartIntervals.indexOf(interval)]?.data;
	if (results.some((result) => result.data === undefined)) return { failed };
	return {
		catalogs: byInterval((interval) => catalog(interval)?.catalog ?? []),
		failed,
		indicators: byInterval(
			(interval) => catalog(interval)?.layout ?? createIndicatorLayout([]),
		),
	};
}

function byInterval<T>(
	value: (interval: CandleInterval) => T,
): Record<CandleInterval, T> {
	return {
		"1h": value("1h"),
		"1d": value("1d"),
		"1w": value("1w"),
		"1M": value("1M"),
	};
}

// A pane needs a pane key and a scale; the backend guarantees them, so a
// missing one is a contract violation rather than something to default.
function validateCatalog(catalog: readonly ChartIndicatorDefinition[]): void {
	if (
		catalog.some(
			({ pane, placement, scale }) => placement === "pane" && (!pane || !scale),
		)
	) {
		throw unexpectedApiError();
	}
}
