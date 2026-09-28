import {
	CandlestickSeries,
	type CandlestickSeriesPartialOptions,
	type IRange,
} from "lightweight-charts";
import { useMemo } from "react";
import { Series } from "@/components/lightweight-chart";
import {
	candlePriceScaleOptions,
	candleSeriesOptions,
} from "@/components/price-history-chart/config";
import { MinMaxPriceLines } from "@/components/price-history-chart/min-max-price-lines";
import type {
	ChartCandle,
	ChartCandleSlot,
} from "@/components/price-history-chart/types";
import {
	chartPriceResolution,
	formatPrice,
	getVisibleMinMax,
	isChartCandle,
} from "@/components/price-history-chart/utils";

interface CandleSeriesProps {
	data: readonly ChartCandleSlot[];
	onActiveCandleChange(candle: ChartCandle | null): void;
	visibleRange: IRange<number> | null;
}

export function CandleSeries({
	data,
	onActiveCandleChange,
	visibleRange,
}: CandleSeriesProps) {
	const options = useMemo<CandlestickSeriesPartialOptions>(() => {
		const { base, fractionDigits, minMove } = chartPriceResolution(data);
		return {
			...candleSeriesOptions,
			priceFormat: {
				base,
				// Axis ticks carry floating-point noise near zero; round it at the
				// chart resolution so it cannot widen the price scale.
				formatter: (value: number) => formatPrice(value, fractionDigits),
				minMove,
				type: "custom",
			},
		};
	}, [data]);
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
		</Series>
	);
}
