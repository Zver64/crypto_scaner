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
import {
	fitPaneIndicatorScale,
	valueResolution,
} from "@/components/price-history-chart/utils";
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
	const { base, fractionDigits, minMove } = valueResolution(
		data.flatMap((slot) => ("value" in slot ? [slot.value] : [])),
	);
	// Depends on the resolution, not on data, so live ticks keep the same
	// options and the series is not reconfigured on every trade.
	const options = useMemo<LineSeriesPartialOptions>(() => {
		return {
			...paneIndicatorSeriesOptions,
			// Fit the visible values instead of the full range, such as RSI 0-100,
			// so the line fills the pane without empty space above and below.
			autoscaleInfoProvider: (baseImplementation: () => AutoscaleInfo | null) =>
				fitPaneIndicatorScale(baseImplementation(), scale),
			color: line.color,
			priceFormat: {
				base,
				// Axis ticks carry floating-point noise near zero; round it at
				// the resolution so it cannot widen the scale.
				formatter: (value: number) => formatNumber(value, fractionDigits),
				minMove,
				type: "custom",
			},
		};
	}, [base, fractionDigits, line, minMove, scale]);
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
