import {
	type CandlestickData,
	CandlestickSeries as CandlestickSeriesDefinition,
	type CandlestickSeriesPartialOptions,
	type DeepPartial,
	type PriceScaleOptions,
	type Time,
	type WhitespaceData,
} from "lightweight-charts";
import type { ReactNode } from "react";
import { SeriesContext } from "@/components/lightweight-chart/context";
import {
	type SeriesCrosshairHandler,
	useSeries,
} from "@/components/lightweight-chart/use-series";

interface CandlestickSeriesProps {
	children?: ReactNode;
	data: readonly (CandlestickData<Time> | WhitespaceData<Time>)[];
	onBeforeDataChange?(): void;
	onCrosshairMove?: SeriesCrosshairHandler<"Candlestick">;
	options?: CandlestickSeriesPartialOptions;
	pane?: number;
	priceScaleOptions?: DeepPartial<PriceScaleOptions>;
}

export function CandlestickSeries({
	children,
	data,
	onBeforeDataChange,
	onCrosshairMove,
	options,
	pane = 0,
	priceScaleOptions,
}: CandlestickSeriesProps) {
	const binding = useSeries({
		data,
		definition: CandlestickSeriesDefinition,
		onBeforeDataChange,
		onCrosshairMove,
		options,
		pane,
		priceScaleOptions,
	});
	return (
		<SeriesContext.Provider value={binding.context}>
			{children}
		</SeriesContext.Provider>
	);
}
