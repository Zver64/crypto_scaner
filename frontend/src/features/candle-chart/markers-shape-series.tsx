import { LineSeries } from "lightweight-charts";

import { Series, SeriesMarkers } from "@/components/lightweight-chart";
import { carrierSeriesOptions } from "@/features/candle-chart/config";
import type { ChartMarkersShapeData } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";
import { lineMarkers } from "@/features/candle-chart/utils";

interface MarkersShapeSeriesProps {
	shape: ChartMarkersShapeData;
}

// The markers of a shape that sit on a line, on a series of the line values
// that draws nothing itself; the candles draw the other markers.
export function MarkersShapeSeries({ shape }: MarkersShapeSeriesProps) {
	const themeColor = useThemeColor();
	const { autoscaleInfoProvider, onLine, pane, placement } = shape;
	return (
		<Series
			data={onLine}
			definition={LineSeries}
			options={{
				...carrierSeriesOptions[placement],
				...(autoscaleInfoProvider ? { autoscaleInfoProvider } : {}),
			}}
			pane={pane}
		>
			<SeriesMarkers markers={lineMarkers(shape, themeColor)} />
		</Series>
	);
}
