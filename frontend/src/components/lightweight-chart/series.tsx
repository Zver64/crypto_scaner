import type {
	DeepPartial,
	PriceScaleOptions,
	SeriesDataItemTypeMap,
	SeriesDefinition,
	SeriesPartialOptionsMap,
	SeriesType,
	Time,
} from "lightweight-charts";
import type { ReactNode } from "react";
import { SeriesContext } from "@/components/lightweight-chart/context";
import { useSeries } from "@/components/lightweight-chart/use-series";

interface SeriesProps<T extends SeriesType> {
	children?: ReactNode;
	// Compared by reference, then by value only when a new array arrives: large
	// data should come from a boundary that creates a new array only when it
	// changes. Options and the price scale are compared by value.
	data: readonly SeriesDataItemTypeMap<Time>[T][];
	definition: SeriesDefinition<T>;
	onBeforeDataChange?(): void;
	onCrosshairMove?(value: SeriesDataItemTypeMap<Time>[T] | undefined): void;
	options: SeriesPartialOptionsMap[T];
	pane?: number;
	priceScale?: DeepPartial<PriceScaleOptions>;
}

export function Series<T extends SeriesType>({
	children,
	pane = 0,
	...options
}: SeriesProps<T>) {
	const context = useSeries({ ...options, pane });
	return (
		<SeriesContext.Provider value={context}>{children}</SeriesContext.Provider>
	);
}
