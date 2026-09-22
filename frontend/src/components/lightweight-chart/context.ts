import type {
	IChartApi,
	IPriceLine,
	ISeriesApi,
	SeriesType,
} from "lightweight-charts";
import { createContext, useContext } from "react";

export interface ChartLifecycle {
	readonly isRemoved: boolean;
	api(): IChartApi;
	applyPaneStretchFactors(): void;
	destroy(): void;
	removeSeries(series: ISeriesApi<SeriesType>): void;
}

export interface SeriesLifecycle {
	readonly isRemoved: boolean;
	api(): ISeriesApi<SeriesType>;
	destroy(): void;
	removePriceLine(priceLine: IPriceLine): void;
}

export const ChartContext = createContext<ChartLifecycle | null>(null);
export const SeriesContext = createContext<SeriesLifecycle | null>(null);

export function useChartLifecycle(): ChartLifecycle {
	const lifecycle = useContext(ChartContext);
	if (lifecycle === null) {
		throw new Error(
			"A Lightweight Charts series must be inside LightweightChart",
		);
	}
	return lifecycle;
}

export function useSeriesLifecycle(): SeriesLifecycle {
	const lifecycle = useContext(SeriesContext);
	if (lifecycle === null) {
		throw new Error("PriceLine must be inside a Lightweight Charts series");
	}
	return lifecycle;
}
