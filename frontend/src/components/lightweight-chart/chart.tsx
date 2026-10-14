import equal from "fast-deep-equal";
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
	ChartCanvasHandle,
	ChartCanvasProps,
} from "@/components/lightweight-chart/types";

export const ChartCanvas = forwardRef<ChartCanvasHandle, ChartCanvasProps>(
	function ChartCanvas(
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
		// The options the chart has: it is created with them and applies only
		// options that differ by value, so callers may build them on every render.
		const optionsRef = useRef(options);
		const rangeChangeRef = useRef(onVisibleLogicalRangeChange);

		const lifecycleRef = useRef<ChartLifecycle | null>(null);
		if (lifecycleRef.current === null) {
			let chartApi: ReturnType<typeof createChart> | null = null;
			let isRemoved = false;
			let generation = 0;
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
						generation++;
						chartApi = createChart(container, {
							...optionsRef.current,
							height: container.clientHeight,
							width: container.clientWidth,
						});
					}
					return chartApi;
				},
				generation: () => generation,
				// Compared with the factors the chart's panes have, not with the
				// last applied ones: a pane recreated by its series remounting
				// starts with the default factor although the factors are equal.
				// Setting a factor redraws the whole chart, so equal ones are skipped.
				applyPaneStretchFactors(factors) {
					if (chartApi === null || isRemoved) return;
					const panes = chartApi.panes();
					const changes = factors.flatMap((factor, index) => {
						const pane = panes[index];
						return pane && pane.getStretchFactor() !== factor
							? [{ factor, pane }]
							: [];
					});
					for (const { factor, pane } of changes) pane.setStretchFactor(factor);
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
				generation: () => lifecycle.generation(),
				containerWidth: () => containerRef.current?.clientWidth ?? 0,
				fitContent: () => lifecycle.api().timeScale().fitContent(),
				getVisibleLogicalRange: () =>
					lifecycle.api().timeScale().getVisibleLogicalRange(),
				setVisibleLogicalRange: (range) =>
					lifecycle.api().timeScale().setVisibleLogicalRange(range),
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
			rangeChangeRef.current = onVisibleLogicalRangeChange;
			if (equal(optionsRef.current, options)) return;
			optionsRef.current = options;
			lifecycle.api().applyOptions(options);
		});

		// Runs after every render, since series added by children create their
		// panes; it changes the chart only when a pane's factor differs.
		useLayoutEffect(() => {
			lifecycle.applyPaneStretchFactors(paneStretchFactors);
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
	},
);
