import { useComputedColorScheme, useMantineTheme } from "@mantine/core";
import { useQueries } from "@tanstack/react-query";
import { useMemo } from "react";
import { getListChartIndicatorsQueryOptions } from "@/api/generated/api";
import type {
	CandleInterval,
	ChartIndicatorDefinition,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { toChartIndicatorOptions } from "@/features/candle-chart/utils";
import {
	type ChartCatalogs,
	chartIntervals,
} from "@/features/instrument-analysis/live-candle-store";

// The administrator configures which indicators the charts of each interval
// show; the backend serves them with how to draw them. The catalogs drive both
// the live chart subscriptions and the rendering.
export function useChartIndicators(enabled: boolean) {
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	const { catalogs, failed } = useQueries({
		queries: chartIntervals.map((interval) =>
			getListChartIndicatorsQueryOptions(
				{ interval },
				{
					query: {
						enabled,
						select: (response) => validateCatalog(response.data.items),
						staleTime: 60_000,
					},
				},
			),
		),
		combine: combineCatalogs,
	});
	const indicators = useMemo(
		() =>
			byInterval((interval) =>
				(catalogs?.[interval] ?? []).map((definition) =>
					toChartIndicatorOptions(definition, theme, colorScheme),
				),
			),
		[catalogs, colorScheme, theme],
	);
	return { catalogs, failed, indicators };
}

// Defined at module level so the combined result keeps its reference while the
// query results do not change.
function combineCatalogs(
	results: readonly { data?: ChartIndicatorDefinition[]; isError: boolean }[],
): { catalogs?: ChartCatalogs; failed: boolean } {
	const failed = results.some((result) => result.isError);
	if (results.some((result) => result.data === undefined)) return { failed };
	return {
		catalogs: byInterval(
			(interval) => results[chartIntervals.indexOf(interval)]?.data ?? [],
		),
		failed,
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
function validateCatalog(
	catalog: ChartIndicatorDefinition[],
): ChartIndicatorDefinition[] {
	if (
		catalog.some(
			({ pane, placement, scale }) => placement === "pane" && (!pane || !scale),
		)
	) {
		throw unexpectedApiError();
	}
	return catalog;
}
