import { LineSeries } from "lightweight-charts";
import { useSyncExternalStore } from "react";

import { Series } from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import {
	candlePriceScaleOptions,
	comparisonSeriesOptions,
} from "@/features/candle-chart/config";
import type {
	ChartInterval,
	PriceComparison,
} from "@/features/candle-chart/types";

interface ComparisonSeriesProps {
	comparison: PriceComparison;
	interval: ChartInterval;
}

// The compared price as a line over the candles, on the left price scale.
export function ComparisonSeries({
	comparison,
	interval,
}: ComparisonSeriesProps) {
	const { data, resolution } = useSyncExternalStore(comparison.subscribe, () =>
		comparison.getLine(interval),
	);
	return (
		<Series
			data={data}
			definition={LineSeries}
			options={comparisonSeriesOptions}
			priceScale={candlePriceScaleOptions}
		>
			<AxisPriceFormat resolution={resolution} />
		</Series>
	);
}
