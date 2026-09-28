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
