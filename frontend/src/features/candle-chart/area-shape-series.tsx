import { LineSeries } from "lightweight-charts";

import { Series, SeriesFill } from "@/components/lightweight-chart";
import { carrierSeriesOptions } from "@/features/candle-chart/config";
import type { ChartAreaShapeData } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";
import { areaFillPoints } from "@/features/candle-chart/utils";

interface AreaShapeSeriesProps {
	shape: ChartAreaShapeData;
}

// An area shape: a translucent fill behind the series of its pane, lying on
// a series of its values that draws nothing itself.
export function AreaShapeSeries({
	shape: { autoscaleInfoProvider, pane, placement, points, values },
}: AreaShapeSeriesProps) {
	const themeColor = useThemeColor();
	return (
		<Series
			data={values}
			definition={LineSeries}
			options={{
				...carrierSeriesOptions[placement],
				...(autoscaleInfoProvider ? { autoscaleInfoProvider } : {}),
			}}
			pane={pane}
		>
			<SeriesFill points={areaFillPoints(points, themeColor)} />
		</Series>
	);
}
