import { useComputedColorScheme } from "@mantine/core";
import { LineSeries, type LineSeriesPartialOptions } from "lightweight-charts";
import { useMemo } from "react";
import { PriceLine, Series } from "@/components/lightweight-chart";
import {
	paneIndicatorLevelOptions,
	paneIndicatorPriceScaleOptions,
	paneIndicatorSeriesOptions,
} from "@/components/price-history-chart/config";
import type {
	ChartIndicatorLine,
	ChartIndicatorScale,
	ChartIndicatorSlot,
} from "@/components/price-history-chart/types";
import { formatNumber } from "@/utils/number-format";

interface PaneIndicatorSeriesProps {
	// The first line also draws the scale levels, which belong to the pane.
	isFirstLine: boolean;
	data: readonly ChartIndicatorSlot[];
	line: ChartIndicatorLine;
	pane: number;
	scale: ChartIndicatorScale;
}

// One indicator line in its own pane below the candles.
export function PaneIndicatorSeries({
	isFirstLine,
	data,
	line,
	pane,
	scale,
}: PaneIndicatorSeriesProps) {
	const levelOptions =
		paneIndicatorLevelOptions[useComputedColorScheme("dark")];
	const options = useMemo<LineSeriesPartialOptions>(() => {
		const { max, min, precision } = scale;
		return {
			...paneIndicatorSeriesOptions,
			// Fixed bounds keep bounded oscillators such as RSI on a stable scale.
			autoscaleInfoProvider:
				min !== undefined && max !== undefined
					? () => ({ priceRange: { maxValue: max, minValue: min } })
					: undefined,
			color: line.color,
			priceFormat: {
				formatter: (value: number) => formatNumber(value, precision),
				minMove: 10 ** -precision,
				type: "custom",
			},
		};
	}, [line, scale]);
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={options}
			pane={pane}
			priceScale={paneIndicatorPriceScaleOptions}
		>
			{isFirstLine
				? scale.levels.map(({ value }) => (
						<PriceLine
							key={value}
							options={{ ...levelOptions, price: value }}
						/>
					))
				: null}
		</Series>
	);
}
