import { useComputedColorScheme } from "@mantine/core";
import { LineSeries } from "lightweight-charts";

import { PriceLine, Series } from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import {
	lineShapeSeriesOptions,
	paneIndicatorLevelOptions,
	paneIndicatorPriceScaleOptions,
} from "@/features/candle-chart/config";
import type { ChartLineShapeData } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";
import { lineSeriesData } from "@/features/candle-chart/utils";

interface LineShapeSeriesProps {
	shape: ChartLineShapeData;
}

// A line shape in its pane, on the scale of its placement, with the levels of
// its pane when it draws them.
export function LineShapeSeries({ shape }: LineShapeSeriesProps) {
	const levelOptions =
		paneIndicatorLevelOptions[useComputedColorScheme("dark")];
	const themeColor = useThemeColor();
	const { autoscaleInfoProvider, color, levels, pane, placement, resolution } =
		shape;
	return (
		<Series
			data={lineSeriesData(shape, themeColor)}
			definition={LineSeries}
			options={{
				...lineShapeSeriesOptions[placement],
				...(autoscaleInfoProvider ? { autoscaleInfoProvider } : {}),
				color: themeColor(color),
			}}
			pane={pane}
			priceScale={
				placement === "pane" ? paneIndicatorPriceScaleOptions : undefined
			}
		>
			{/* Volume lines keep the format of the volume bars. */}
			{placement === "volume" ? null : (
				<AxisPriceFormat resolution={resolution} />
			)}
			{levels.map(({ value }) => (
				<PriceLine key={value} options={{ ...levelOptions, price: value }} />
			))}
		</Series>
	);
}
