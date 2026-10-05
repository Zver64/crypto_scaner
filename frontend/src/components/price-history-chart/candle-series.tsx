import {
	CandlestickSeries,
	type CandlestickSeriesPartialOptions,
	type IRange,
	type PriceFormatCustom,
} from "lightweight-charts";
import { useMemo } from "react";
import { Series, SeriesMarkers } from "@/components/lightweight-chart";
import {
	candlePriceScaleOptions,
	candleSeriesOptions,
} from "@/components/price-history-chart/config";
import { MinMaxPriceLines } from "@/components/price-history-chart/min-max-price-lines";
import type {
	ChartCandle,
	ChartCandleSlot,
	ChartMarker,
} from "@/components/price-history-chart/types";
import {
	getVisibleMinMax,
	isChartCandle,
} from "@/components/price-history-chart/utils";

interface CandleSeriesProps {
	data: readonly ChartCandleSlot[];
	markers?: readonly ChartMarker[];
	priceFormat: PriceFormatCustom;
	onActiveCandleChange(candle: ChartCandle | null): void;
	visibleRange: IRange<number> | null;
}

export function CandleSeries({
	data,
	markers,
	onActiveCandleChange,
	priceFormat,
	visibleRange,
}: CandleSeriesProps) {
	const options = useMemo<CandlestickSeriesPartialOptions>(
		() => ({ ...candleSeriesOptions, priceFormat }),
		[priceFormat],
	);
	const minMax = useMemo(
		() => getVisibleMinMax(data, visibleRange),
		[data, visibleRange],
	);
	return (
		<Series
			data={data}
			definition={CandlestickSeries}
			onCrosshairMove={(value) =>
				onActiveCandleChange(isChartCandle(value) ? value : null)
			}
			options={options}
			priceScale={candlePriceScaleOptions}
		>
			{minMax ? <MinMaxPriceLines max={minMax.max} min={minMax.min} /> : null}
			{markers ? <SeriesMarkers markers={markers} /> : null}
		</Series>
	);
}
