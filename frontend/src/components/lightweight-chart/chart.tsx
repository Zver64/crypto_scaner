import { createChart } from "lightweight-charts";
import {
	forwardRef,
	useImperativeHandle,
	useLayoutEffect,
	useRef,
} from "react";
import {
	ChartContext,
	type ChartLifecycle,
} from "@/components/lightweight-chart/context";
import type {
	LightweightChartHandle,
	LightweightChartProps,
} from "@/components/lightweight-chart/types";

export const LightweightChart = forwardRef<
	LightweightChartHandle,
	LightweightChartProps
>(function LightweightChart(
	{
		children,
		onVisibleLogicalRangeChange,
		options,
		paneStretchFactors = [],
		...containerProps
	},
	ref,
) {
	const containerRef = useRef<HTMLDivElement>(null);
	const optionsRef = useRef(options);
	const paneStretchFactorsRef = useRef(paneStretchFactors);
	const rangeChangeRef = useRef(onVisibleLogicalRangeChange);
	optionsRef.current = options;
	paneStretchFactorsRef.current = paneStretchFactors;
	rangeChangeRef.current = onVisibleLogicalRangeChange;

	const lifecycleRef = useRef<ChartLifecycle | null>(null);
	if (lifecycleRef.current === null) {
		let chartApi: ReturnType<typeof createChart> | null = null;
		let isRemoved = false;
		lifecycleRef.current = {
			get isRemoved() {
				return isRemoved;
			},
			api() {
				if (chartApi === null) {
					const container = containerRef.current;
					if (container === null) {
						throw new Error("LightweightChart container is not mounted");
					}
					isRemoved = false;
					chartApi = createChart(container, {
						...optionsRef.current,
						height: container.clientHeight,
						width: container.clientWidth,
					});
				}
				return chartApi;
			},
			applyPaneStretchFactors() {
				if (chartApi === null || isRemoved) return;
				for (const [index, factor] of paneStretchFactorsRef.current.entries()) {
					chartApi.panes()[index]?.setStretchFactor(factor);
				}
			},
			destroy() {
				isRemoved = true;
				const current = chartApi;
				chartApi = null;
				current?.remove();
			},
			removeSeries(series) {
				if (chartApi !== null && !isRemoved) chartApi.removeSeries(series);
			},
		};
	}
	const lifecycle = lifecycleRef.current;

	useImperativeHandle(
		ref,
		() => ({
			api: () => lifecycle.api(),
			containerWidth: () => containerRef.current?.clientWidth ?? 0,
		}),
		[lifecycle],
	);

	useLayoutEffect(() => {
		const chart = lifecycle.api();
		const container = containerRef.current;
		if (container === null) return;
		const resize = () => {
			const { clientHeight, clientWidth } = container;
			if (clientHeight > 0 && clientWidth > 0) {
				chart.resize(clientWidth, clientHeight);
			}
		};
		const observer = new ResizeObserver(resize);
		observer.observe(container);
		resize();
		return () => {
			observer.disconnect();
			lifecycle.destroy();
		};
	}, [lifecycle]);

	useLayoutEffect(() => {
		if (options !== undefined) lifecycle.api().applyOptions(options);
	}, [lifecycle, options]);

	useLayoutEffect(() => {
		lifecycle.applyPaneStretchFactors();
	});

	useLayoutEffect(() => {
		const chart = lifecycle.api();
		const timeScale = chart.timeScale();
		const handleRangeChange: Parameters<
			typeof timeScale.subscribeVisibleLogicalRangeChange
		>[0] = (range) => rangeChangeRef.current?.(range);
		timeScale.subscribeVisibleLogicalRangeChange(handleRangeChange);
		return () => {
			if (!lifecycle.isRemoved) {
				timeScale.unsubscribeVisibleLogicalRangeChange(handleRangeChange);
			}
		};
	}, [lifecycle]);

	return (
		<ChartContext.Provider value={lifecycle}>
			<div ref={containerRef} {...containerProps} />
			{children}
		</ChartContext.Provider>
	);
});
