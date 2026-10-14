import { LineSeries } from "lightweight-charts";
import { Series } from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import { overlayIndicatorSeriesOptions } from "@/features/candle-chart/config";
import type {
	ChartIndicatorLine,
	ChartIndicatorSlot,
	ValueResolution,
} from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface OverlayIndicatorSeriesProps {
	data: readonly ChartIndicatorSlot[];
	line: ChartIndicatorLine;
	// The resolution of the candles, so all labels on their scale look the same.
	priceResolution: ValueResolution;
}

// One indicator line drawn over the candles, on their price scale.
export function OverlayIndicatorSeries({
	data,
	line,
	priceResolution,
}: OverlayIndicatorSeriesProps) {
	const themeColor = useThemeColor();
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={{
				...overlayIndicatorSeriesOptions,
				color: themeColor(line.color),
			}}
		>
			<AxisPriceFormat resolution={priceResolution} />
		</Series>
	);
}
