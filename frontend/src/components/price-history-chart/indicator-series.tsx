import { useComputedColorScheme } from "@mantine/core";
import { LineSeries, type LineSeriesPartialOptions } from "lightweight-charts";
import { useMemo } from "react";
import { PriceLine, Series } from "@/components/lightweight-chart";
import {
	indicatorPriceLineOptions,
	indicatorPriceScaleOptions,
	indicatorSeriesOptions,
} from "@/components/price-history-chart/config";
import type {
	ChartIndicatorOptions,
	ChartIndicatorSlot,
} from "@/components/price-history-chart/types";

interface IndicatorSeriesProps {
	data: readonly ChartIndicatorSlot[];
	indicator: ChartIndicatorOptions;
}

export function IndicatorSeries({ data, indicator }: IndicatorSeriesProps) {
	const lineOptions = indicatorPriceLineOptions[useComputedColorScheme("dark")];
	const options = useMemo<LineSeriesPartialOptions>(
		() => ({
			...indicatorSeriesOptions,
			autoscaleInfoProvider: () => ({
				priceRange: {
					maxValue: indicator.bounds.max,
					minValue: indicator.bounds.min,
				},
			}),
			priceFormat: {
				formatter: indicator.formatValue,
				minMove: indicator.minMove,
				type: "custom",
			},
		}),
		[indicator],
	);
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={options}
			pane={1}
			priceScale={indicatorPriceScaleOptions}
		>
			{indicator.lines.map(({ price, title }) => (
				<PriceLine key={price} options={{ ...lineOptions, price, title }} />
			))}
		</Series>
	);
}
