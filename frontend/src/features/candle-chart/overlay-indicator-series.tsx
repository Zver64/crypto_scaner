import { LineSeries } from "lightweight-charts";

import {
	Series,
	SeriesFill,
	SeriesMarkers,
} from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import { overlayIndicatorSeriesOptions } from "@/features/candle-chart/config";
import type {
	ChartIndicatorLine,
	ChartLineData,
	ChartMarker,
	ValueResolution,
} from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";
import { colorByDirection, directionFill } from "@/features/candle-chart/utils";

interface OverlayIndicatorSeriesProps {
	data: ChartLineData;
	line: ChartIndicatorLine;
	// The markers of the lines anchored to this one.
	markers?: readonly ChartMarker[];
	// The resolution of the candles, so all labels on their scale look the same.
	priceResolution: ValueResolution;
}

// One indicator line drawn over the candles, on their price scale.
export function OverlayIndicatorSeries({
	data,
	line,
	markers,
	priceResolution,
}: OverlayIndicatorSeriesProps) {
	const themeColor = useThemeColor();
	return (
		<Series
			data={colorByDirection(data, line.direction, themeColor)}
			definition={LineSeries}
			options={{
				...overlayIndicatorSeriesOptions,
				color: themeColor(line.color),
			}}
		>
			<AxisPriceFormat resolution={priceResolution} />
			{markers ? <SeriesMarkers markers={markers} /> : null}
			{line.direction?.fill ? (
				<SeriesFill points={directionFill(data, line.direction, themeColor)} />
			) : null}
		</Series>
	);
}
