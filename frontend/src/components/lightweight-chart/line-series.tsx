import {
	LineSeries as LineSeriesDefinition,
	type UTCTimestamp,
} from "lightweight-charts";
import { type ReactNode, useMemo } from "react";
import { SeriesContext } from "@/components/lightweight-chart/context";
import type {
	ChartIndicatorSlot,
	ChartLineOptions,
} from "@/components/lightweight-chart/types";
import { useSeries } from "@/components/lightweight-chart/use-series";

const priceScaleOptions = {
	autoScale: true,
	scaleMargins: { bottom: 0, top: 0 },
};

interface LineSeriesProps {
	children?: ReactNode;
	data: readonly ChartIndicatorSlot[];
	options: ChartLineOptions;
	pane?: number;
}

export function LineSeries({
	children,
	data,
	options,
	pane = 0,
}: LineSeriesProps) {
	const seriesData = useMemo(
		() => data.map((item) => ({ ...item, time: item.time as UTCTimestamp })),
		[data],
	);
	const seriesOptions = useMemo(
		() => ({
			autoscaleInfoProvider: () => ({
				priceRange: {
					maxValue: options.bounds.max,
					minValue: options.bounds.min,
				},
			}),
			color: options.color,
			lastValueVisible: true,
			lineWidth: 2 as const,
			priceFormat: {
				formatter: options.formatValue,
				minMove: options.minMove,
				type: "custom" as const,
			},
			priceLineVisible: false,
		}),
		[options],
	);
	const binding = useSeries({
		data: seriesData,
		definition: LineSeriesDefinition,
		options: seriesOptions,
		pane,
		priceScaleOptions,
	});
	return (
		<SeriesContext.Provider value={binding.context}>
			{children}
		</SeriesContext.Provider>
	);
}
