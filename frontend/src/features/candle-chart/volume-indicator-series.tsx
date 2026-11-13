import { LineSeries } from "lightweight-charts";

import { Series } from "@/components/lightweight-chart";
import { volumeIndicatorSeriesOptions } from "@/features/candle-chart/config";
import type {
	ChartIndicatorLine,
	ChartIndicatorSlot,
} from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface VolumeIndicatorSeriesProps {
	data: readonly ChartIndicatorSlot[];
	line: ChartIndicatorLine;
}

// One indicator line of volumes drawn over the volume bars, on their scale.
export function VolumeIndicatorSeries({
	data,
	line,
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
		/>
	);
}
