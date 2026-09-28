import { useComputedColorScheme } from "@mantine/core";
import {
	type AutoscaleInfo,
	LineSeries,
	type LineSeriesPartialOptions,
} from "lightweight-charts";
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
import { fitPaneIndicatorScale } from "@/components/price-history-chart/utils";
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
		const { precision } = scale;
		return {
			...paneIndicatorSeriesOptions,
			// Fit the visible values instead of the full range, such as RSI 0-100,
			// so the line fills the pane without empty space above and below.
			autoscaleInfoProvider: (baseImplementation: () => AutoscaleInfo | null) =>
				fitPaneIndicatorScale(baseImplementation(), scale),
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
