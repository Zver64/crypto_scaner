import {
	type MantineTheme,
	parseThemeColor,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";
import { useMemo } from "react";
import { useListChartIndicators } from "@/api/generated/api";
import type { ChartIndicatorDefinition } from "@/api/generated/models";
import { telegramRequestOptions } from "@/app/telegram";
import type { ChartIndicatorOptions } from "@/components/price-history-chart";
import { unexpectedApiError } from "@/features/analysis/api-error";

// The backend owns which indicators charts show and how to draw them. The
// catalog drives both the live chart subscription and the rendering.
export function useChartIndicators(enabled: boolean) {
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	const { data: catalog, isError } = useListChartIndicators({
		fetch: telegramRequestOptions(),
		query: {
			enabled,
			select: (response) => validateCatalog(response.data.items),
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	const indicators = useMemo(
		() =>
			(catalog ?? []).map((definition) =>
				toChartIndicatorOptions(definition, theme, colorScheme),
			),
		[catalog, colorScheme, theme],
	);
	return { catalog, failed: isError, indicators };
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
				scale: scale ?? { levels: [], precision: 0 },
			};
}
