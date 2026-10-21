import { CandlestickSeries, type IRange } from "lightweight-charts";

import { Series, SeriesMarkers } from "@/components/lightweight-chart";
import { AxisPriceFormat } from "@/features/candle-chart/axis-price-format";
import {
	candlePriceScaleOptions,
	candleSeriesOptions,
} from "@/features/candle-chart/config";
import { MinMaxPriceLines } from "@/features/candle-chart/min-max-price-lines";
import { PriceLevelLines } from "@/features/candle-chart/price-level-lines";
import type {
	ChartCandle,
	ChartCandleSlot,
	ChartMarker,
	ChartPriceLevels,
	ValueResolution,
} from "@/features/candle-chart/types";
import { getVisibleMinMax, isChartCandle } from "@/features/candle-chart/utils";

interface CandleSeriesProps {
	// The close of the latest candle, which colors the price level steps.
	currentPrice?: number;
	data: readonly ChartCandleSlot[];
	markers?: readonly ChartMarker[];
	priceLevels?: ChartPriceLevels;
	priceResolution: ValueResolution;
	onActiveCandleChange(candle: ChartCandle | null): void;
	visibleRange: IRange<number> | null;
}

export function CandleSeries({
	currentPrice,
	data,
	markers,
	onActiveCandleChange,
	priceLevels,
	priceResolution,
	visibleRange,
}: CandleSeriesProps) {
	const minMax = getVisibleMinMax(data, visibleRange);
	// The levels are colored by the current price, so they wait for a candle.
	const shownLevels = currentPrice === undefined ? undefined : priceLevels;
	return (
		<Series
			data={data}
			definition={CandlestickSeries}
			onCrosshairMove={(value) =>
				onActiveCandleChange(isChartCandle(value) ? value : null)
			}
			options={{
				...candleSeriesOptions,
				// Present even when undefined, so removed levels reset the autoscale.
				autoscaleInfoProvider: shownLevels?.autoscaleInfoProvider,
			}}
			priceScale={candlePriceScaleOptions}
		>
			<AxisPriceFormat resolution={priceResolution} />
			{minMax ? <MinMaxPriceLines max={minMax.max} min={minMax.min} /> : null}
			{shownLevels && currentPrice !== undefined ? (
				<PriceLevelLines
					currentPrice={currentPrice}
					levels={shownLevels.levels}
				/>
			) : null}
			{markers ? <SeriesMarkers markers={markers} /> : null}
		</Series>
	);
}
