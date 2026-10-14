import { useComputedColorScheme } from "@mantine/core";
import { type AutoscaleInfoProvider, LineSeries } from "lightweight-charts";

import { PriceLine, Series } from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import {
	paneIndicatorLevelOptions,
	paneIndicatorPriceScaleOptions,
	paneIndicatorSeriesOptions,
} from "@/features/candle-chart/config";
import type {
	ChartIndicatorLine,
	ChartIndicatorScale,
	ChartLineData,
} from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface PaneIndicatorSeriesProps {
	autoscaleInfoProvider: AutoscaleInfoProvider;
	// One line of each pane also draws the scale levels, which belong to the pane.
	drawsLevels: boolean;
	data: ChartLineData;
	line: ChartIndicatorLine;
	pane: number;
	scale: ChartIndicatorScale;
}

// One indicator line in a pane below the candles, on the scale of the pane.
export function PaneIndicatorSeries({
	autoscaleInfoProvider,
	drawsLevels,
	data: { data, resolution },
	line,
	pane,
	scale,
}: PaneIndicatorSeriesProps) {
	const levelOptions =
		paneIndicatorLevelOptions[useComputedColorScheme("dark")];
	const themeColor = useThemeColor();
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={{
				...paneIndicatorSeriesOptions,
				autoscaleInfoProvider,
				color: themeColor(line.color),
			}}
			pane={pane}
			priceScale={paneIndicatorPriceScaleOptions}
		>
			<AxisPriceFormat resolution={resolution} />
			{drawsLevels
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
