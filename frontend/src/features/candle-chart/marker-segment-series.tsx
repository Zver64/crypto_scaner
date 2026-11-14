import { LineSeries } from "lightweight-charts";

import { Series } from "@/components/lightweight-chart";
import { markerSegmentSeriesOptions } from "@/features/candle-chart/config";
import type { ChartMarkerSegments } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface MarkerSegmentSeriesProps {
	segments: ChartMarkerSegments;
}

// The segments between values of one marker line, in the color of that line.
export function MarkerSegmentSeries({
	segments: { color, data, pane, scale },
}: MarkerSegmentSeriesProps) {
	const themeColor = useThemeColor();
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={{
				...markerSegmentSeriesOptions[scale],
				color: themeColor(color),
			}}
			pane={pane}
		/>
	);
}
