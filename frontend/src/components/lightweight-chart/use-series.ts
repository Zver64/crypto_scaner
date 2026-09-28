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
import { useLayoutEffect, useRef } from "react";
import {
	type SeriesLifecycle,
	useChartLifecycle,
} from "@/components/lightweight-chart/context";

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
	const optionsRef = useRef(options);
	const paneRef = useRef(pane);
	const beforeDataChangeRef = useRef(onBeforeDataChange);
	const crosshairRef = useRef(onCrosshairMove);
	const hasCrosshairHandler = onCrosshairMove !== undefined;
	useLayoutEffect(() => {
		definitionRef.current = definition;
		optionsRef.current = options;
		paneRef.current = pane;
		beforeDataChangeRef.current = onBeforeDataChange;
		crosshairRef.current = onCrosshairMove;
	}, [definition, options, pane, onBeforeDataChange, onCrosshairMove]);

	const bindingRef = useRef<SeriesBinding<T> | null>(null);
	if (bindingRef.current === null) {
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
		bindingRef.current = { context, series };
	}
	const binding = bindingRef.current;

	useLayoutEffect(() => {
		binding.series();
		return () => binding.context.destroy();
	}, [binding]);

	useLayoutEffect(() => {
		binding.series().applyOptions(options);
	}, [binding, options]);

	useLayoutEffect(() => {
		beforeDataChangeRef.current?.();
		binding.series().setData(Array.from(data));
	}, [binding, data]);

	useLayoutEffect(() => {
		const series = binding.series();
		series.moveToPane(pane);
		if (priceScale !== undefined) series.priceScale().applyOptions(priceScale);
	}, [binding, pane, priceScale]);

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
