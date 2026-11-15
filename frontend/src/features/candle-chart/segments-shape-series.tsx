import { LineSeries } from "lightweight-charts";

import { Series } from "@/components/lightweight-chart";
import { segmentsShapeSeriesOptions } from "@/features/candle-chart/config";
import type { ChartSegmentsShapeData } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface SegmentsShapeSeriesProps {
	shape: ChartSegmentsShapeData;
}

// A segments shape in its pane, on the scale of its placement.
export function SegmentsShapeSeries({
	shape: { autoscaleInfoProvider, color, data, pane, placement },
}: SegmentsShapeSeriesProps) {
	const themeColor = useThemeColor();
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={{
				...segmentsShapeSeriesOptions[placement],
				...(autoscaleInfoProvider ? { autoscaleInfoProvider } : {}),
				color: themeColor(color),
			}}
			pane={pane}
		/>
	);
}
