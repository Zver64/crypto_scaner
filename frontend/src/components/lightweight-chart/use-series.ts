import equal from "fast-deep-equal";
import type {
	DeepPartial,
	IPriceLine,
	ISeriesApi,
	PriceScaleOptions,
	SeriesDataItemTypeMap,
	SeriesDefinition,
	SeriesPartialOptionsMap,
	SeriesType,
	Time,
} from "lightweight-charts";
import { useLayoutEffect, useRef, useState } from "react";

import {
	type SeriesLifecycle,
	useChartLifecycle,
} from "@/components/lightweight-chart/context";
import { dataChange } from "@/components/lightweight-chart/utils";

interface UseSeriesOptions<T extends SeriesType> {
	data: readonly SeriesDataItemTypeMap<Time>[T][];
	definition: SeriesDefinition<T>;
	onBeforeDataChange?(): void;
	onCrosshairMove?(value: SeriesDataItemTypeMap<Time>[T] | undefined): void;
	options: SeriesPartialOptionsMap[T];
	pane: number;
	priceScale?: DeepPartial<PriceScaleOptions>;
}

interface SeriesBinding<T extends SeriesType> {
	context: SeriesLifecycle;
	series(): ISeriesApi<T>;
}

export function useSeries<T extends SeriesType>({
	data,
	definition,
	onBeforeDataChange,
	onCrosshairMove,
	options,
	pane,
	priceScale,
}: UseSeriesOptions<T>): SeriesLifecycle {
	const parent = useChartLifecycle();
	const definitionRef = useRef(definition);
	// The options and pane the series has: it is created with them and applies
	// only options that differ by value, so callers may build them on every
	// render.
	const optionsRef = useRef(options);
	const paneRef = useRef(pane);
	const beforeDataChangeRef = useRef(onBeforeDataChange);
	const crosshairRef = useRef(onCrosshairMove);
	const hasCrosshairHandler = onCrosshairMove !== undefined;
	useLayoutEffect(() => {
		definitionRef.current = definition;
		beforeDataChangeRef.current = onBeforeDataChange;
		crosshairRef.current = onCrosshairMove;
	});

	const [binding] = useState((): SeriesBinding<T> => {
		let seriesApi: ISeriesApi<T> | null = null;
		let isRemoved = false;
		const series = () => {
			if (seriesApi === null) {
				isRemoved = false;
				seriesApi = parent
					.api()
					.addSeries(
						definitionRef.current,
						optionsRef.current,
						paneRef.current,
					);
			}
			return seriesApi;
		};
		const context: SeriesLifecycle = {
			get isRemoved() {
				return isRemoved;
			},
			api: () => series() as unknown as ISeriesApi<SeriesType>,
			destroy() {
				isRemoved = true;
				const current = seriesApi;
				seriesApi = null;
				if (current !== null && !parent.isRemoved) {
					parent.removeSeries(current as unknown as ISeriesApi<SeriesType>);
				}
			},
			removePriceLine(priceLine: IPriceLine) {
				if (seriesApi !== null && !isRemoved && !parent.isRemoved) {
					seriesApi.removePriceLine(priceLine);
				}
			},
		};
		return { context, series };
	});

	// Data and price scale options applied to the current series; a recreated
	// series starts empty and with the default price scale.
	const appliedDataRef = useRef<readonly SeriesDataItemTypeMap<Time>[T][]>([]);
	const appliedPriceScaleRef =
		useRef<DeepPartial<PriceScaleOptions>>(undefined);
	useLayoutEffect(() => {
		binding.series();
		return () => {
			binding.context.destroy();
			appliedDataRef.current = [];
			appliedPriceScaleRef.current = undefined;
		};
	}, [binding]);

	useLayoutEffect(() => {
		if (equal(optionsRef.current, options)) return;
		optionsRef.current = options;
		binding.series().applyOptions(options);
	});

	// Data is compared by reference first: the data boundary creates a new
	// array only when the data changes, so large histories are compared only
	// then, in the same pass that detects a live tick.
	useLayoutEffect(() => {
		const applied = appliedDataRef.current;
		if (applied === data) return;
		beforeDataChangeRef.current?.();
		appliedDataRef.current = data;
		const change = dataChange(applied, data);
		if (change === "none") return;
		const series = binding.series();
		const last = data.at(-1);
		// Live ticks only touch the last point; redrawing the whole series on
		// every trade would scale with the loaded history.
		if (change === "tail" && last !== undefined) {
			try {
				series.update(last);
			} catch {
				// update() rejects a point older than the current last one.
				series.setData(Array.from(data));
			}
		} else {
			series.setData(Array.from(data));
		}
	});

	useLayoutEffect(() => {
		const series = binding.series();
		if (paneRef.current !== pane) {
			paneRef.current = pane;
			series.moveToPane(pane);
		}
		if (
			priceScale !== undefined &&
			!equal(appliedPriceScaleRef.current, priceScale)
		) {
			appliedPriceScaleRef.current = priceScale;
			series.priceScale().applyOptions(priceScale);
		}
	});

	useLayoutEffect(() => {
		if (!hasCrosshairHandler) return;
		const chart = parent.api();
		const handleCrosshairMove: Parameters<
			typeof chart.subscribeCrosshairMove
		>[0] = (parameter) => {
			const value = parameter.seriesData.get(binding.series()) as
				| SeriesDataItemTypeMap<Time>[T]
				| undefined;
			crosshairRef.current?.(value);
		};
		chart.subscribeCrosshairMove(handleCrosshairMove);
		return () => {
			if (!parent.isRemoved) {
				chart.unsubscribeCrosshairMove(handleCrosshairMove);
			}
		};
	}, [binding, hasCrosshairHandler, parent]);

	return binding.context;
}
