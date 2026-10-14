import { CandlestickSeries, type IRange } from "lightweight-charts";

import { Series, SeriesMarkers } from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import {
	candlePriceScaleOptions,
	candleSeriesOptions,
} from "@/features/candle-chart/config";
import { MinMaxPriceLines } from "@/features/candle-chart/min-max-price-lines";
import type {
	ChartCandle,
	ChartCandleSlot,
	ChartMarker,
	ValueResolution,
} from "@/features/candle-chart/types";
import { getVisibleMinMax, isChartCandle } from "@/features/candle-chart/utils";

interface CandleSeriesProps {
	data: readonly ChartCandleSlot[];
	markers?: readonly ChartMarker[];
	priceResolution: ValueResolution;
	onActiveCandleChange(candle: ChartCandle | null): void;
	visibleRange: IRange<number> | null;
}

export function CandleSeries({
	data,
	markers,
	onActiveCandleChange,
	priceResolution,
	visibleRange,
}: CandleSeriesProps) {
	const minMax = getVisibleMinMax(data, visibleRange);
	return (
		<Series
			data={data}
			definition={CandlestickSeries}
			onCrosshairMove={(value) =>
				onActiveCandleChange(isChartCandle(value) ? value : null)
			}
			options={candleSeriesOptions}
			priceScale={candlePriceScaleOptions}
		>
			<AxisPriceFormat resolution={priceResolution} />
			{minMax ? <MinMaxPriceLines max={minMax.max} min={minMax.min} /> : null}
			{markers ? <SeriesMarkers markers={markers} /> : null}
		</Series>
	);
}
