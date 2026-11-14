import { LineSeries } from "lightweight-charts";

import { Series, SeriesMarkers } from "@/components/lightweight-chart";
import { volumeIndicatorSeriesOptions } from "@/features/candle-chart/config";
import type {
	ChartIndicatorLine,
	ChartIndicatorSlot,
	ChartMarker,
} from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface VolumeIndicatorSeriesProps {
	data: readonly ChartIndicatorSlot[];
	line: ChartIndicatorLine;
	// The markers of the lines anchored to this one.
	markers?: readonly ChartMarker[];
}

// One indicator line of volumes drawn over the volume bars, on their scale.
export function VolumeIndicatorSeries({
	data,
	line,
	markers,
}: VolumeIndicatorSeriesProps) {
	const themeColor = useThemeColor();
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={{
				...volumeIndicatorSeriesOptions,
				color: themeColor(line.color),
			}}
		>
			{markers ? <SeriesMarkers markers={markers} /> : null}
		</Series>
	);
}
