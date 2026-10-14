import { HistogramSeries } from "lightweight-charts";
import { Series } from "@/components/lightweight-chart";
import {
	volumePriceScaleOptions,
	volumeSeriesOptions,
} from "@/features/candle-chart/config";
import type { ChartVolumeSlot } from "@/features/candle-chart/types";

interface VolumeSeriesProps {
	data: readonly ChartVolumeSlot[];
	onBeforeDataChange(): void;
}

export function VolumeSeries({ data, onBeforeDataChange }: VolumeSeriesProps) {
	return (
		<Series
			data={data}
			definition={HistogramSeries}
			onBeforeDataChange={onBeforeDataChange}
			options={volumeSeriesOptions}
			priceScale={volumePriceScaleOptions}
		/>
	);
}
