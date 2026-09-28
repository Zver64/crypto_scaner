import {
	LineSeries,
	type LineSeriesPartialOptions,
	type PriceFormatCustom,
} from "lightweight-charts";
import { useMemo } from "react";
import { Series } from "@/components/lightweight-chart";
import { overlayIndicatorSeriesOptions } from "@/components/price-history-chart/config";
import type {
	ChartIndicatorLine,
	ChartIndicatorSlot,
} from "@/components/price-history-chart/types";

interface OverlayIndicatorSeriesProps {
	data: readonly ChartIndicatorSlot[];
	line: ChartIndicatorLine;
	priceFormat: PriceFormatCustom;
}

// One indicator line drawn over the candles, on their price scale.
export function OverlayIndicatorSeries({
	data,
	line,
	priceFormat,
}: OverlayIndicatorSeriesProps) {
	const options = useMemo<LineSeriesPartialOptions>(
		() => ({
			...overlayIndicatorSeriesOptions,
			color: line.color,
			priceFormat,
		}),
		[line, priceFormat],
	);
	return <Series data={data} definition={LineSeries} options={options} />;
}
