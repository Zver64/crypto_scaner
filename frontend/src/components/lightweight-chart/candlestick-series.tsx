import {
	CandlestickSeries as CandlestickSeriesDefinition,
	type UTCTimestamp,
} from "lightweight-charts";
import { type ReactNode, useMemo } from "react";
import { SeriesContext } from "@/components/lightweight-chart/context";
import type {
	ChartCandleSlot,
	ChartCandlestick,
	ChartCandlestickOptions,
} from "@/components/lightweight-chart/types";
import { useSeries } from "@/components/lightweight-chart/use-series";

interface CandlestickSeriesProps {
	children?: ReactNode;
	data: readonly ChartCandleSlot[];
	onBeforeDataChange?(): void;
	onCrosshairMove?(value: ChartCandlestick | undefined): void;
	options: ChartCandlestickOptions;
}

export function CandlestickSeries({
	children,
	data,
	onBeforeDataChange,
	onCrosshairMove,
	options,
}: CandlestickSeriesProps) {
	const seriesData = useMemo(
		() => data.map((item) => ({ ...item, time: item.time as UTCTimestamp })),
		[data],
	);
	const seriesOptions = useMemo(
		() => ({
			borderVisible: false,
			downColor: options.downColor,
			upColor: options.upColor,
			wickDownColor: options.downColor,
			wickUpColor: options.upColor,
			priceFormat: {
				type: "custom" as const,
				formatter: options.formatPrice,
				base: options.base,
				minMove: options.minMove,
			},
		}),
		[options],
	);
	const binding = useSeries({
		data: seriesData,
		definition: CandlestickSeriesDefinition,
		onBeforeDataChange,
		onCrosshairMove: onCrosshairMove
			? (value) =>
					onCrosshairMove(
						value && "open" in value && typeof value.time === "number"
							? { ...value, time: value.time }
							: undefined,
					)
			: undefined,
		options: seriesOptions,
		pane: 0,
	});
	return (
		<SeriesContext.Provider value={binding.context}>
			{children}
		</SeriesContext.Provider>
	);
}
