import {
	type MantineTheme,
	parseThemeColor,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";
import { useQueries } from "@tanstack/react-query";
import { useMemo } from "react";
import { getListChartIndicatorsQueryOptions } from "@/api/generated/api";
import type {
	CandleInterval,
	ChartIndicatorDefinition,
} from "@/api/generated/models";
import type { ChartIndicatorOptions } from "@/components/price-history-chart";
import { unexpectedApiError } from "@/features/analysis/api-error";
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

// A pane needs its own scale; the backend guarantees it, so a missing one is a
// contract violation rather than something to default.
function validateCatalog(
	catalog: ChartIndicatorDefinition[],
): ChartIndicatorDefinition[] {
	if (catalog.some(({ placement, scale }) => placement === "pane" && !scale)) {
		throw unexpectedApiError();
	}
	return catalog;
}

function toChartIndicatorOptions(
	{ id, lines, placement, scale }: ChartIndicatorDefinition,
	theme: MantineTheme,
	colorScheme: "light" | "dark",
): ChartIndicatorOptions {
	// Colors arrive as theme tokens such as "yellow.5"; the canvas needs values.
	const resolvedLines = lines.map((line) => ({
		...line,
		color: parseThemeColor({ color: line.color, colorScheme, theme }).value,
	}));
	return placement === "overlay"
		? { id, lines: resolvedLines, placement }
		: {
				id,
				lines: resolvedLines,
				placement,
				// validateCatalog guarantees a scale for every pane.
				scale: scale ?? { levels: [] },
			};
}
