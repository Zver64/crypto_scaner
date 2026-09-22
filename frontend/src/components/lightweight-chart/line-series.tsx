import {
	type DeepPartial,
	type LineData,
	LineSeries as LineSeriesDefinition,
	type LineSeriesPartialOptions,
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

interface LineSeriesProps {
	children?: ReactNode;
	data: readonly (LineData<Time> | WhitespaceData<Time>)[];
	onBeforeDataChange?(): void;
	onCrosshairMove?: SeriesCrosshairHandler<"Line">;
	options?: LineSeriesPartialOptions;
	pane?: number;
	priceScaleOptions?: DeepPartial<PriceScaleOptions>;
}

export function LineSeries({
	children,
	data,
	onBeforeDataChange,
	onCrosshairMove,
	options,
	pane = 0,
	priceScaleOptions,
}: LineSeriesProps) {
	const binding = useSeries({
		data,
		definition: LineSeriesDefinition,
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
