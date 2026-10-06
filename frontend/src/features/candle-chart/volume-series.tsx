import { HistogramSeries } from "lightweight-charts";
import { useMemo } from "react";
import { Series } from "@/components/lightweight-chart";
import {
	volumeColors,
	volumePriceScaleOptions,
	volumeSeriesOptions,
} from "@/features/candle-chart/config";
import type {
	ChartCandleSlot,
	PriceCandle,
} from "@/features/candle-chart/types";
import { createVolumeData } from "@/features/candle-chart/utils";

interface VolumeSeriesProps {
	candles: readonly PriceCandle[];
	data: readonly ChartCandleSlot[];
	onBeforeDataChange(): void;
}

export function VolumeSeries({
	candles,
	data,
	onBeforeDataChange,
}: VolumeSeriesProps) {
	const volumeData = useMemo(
		() => createVolumeData(data, candles, volumeColors),
		[candles, data],
	);
	return (
		<Series
			data={volumeData}
			definition={HistogramSeries}
			onBeforeDataChange={onBeforeDataChange}
			options={volumeSeriesOptions}
			priceScale={volumePriceScaleOptions}
		/>
	);
}
