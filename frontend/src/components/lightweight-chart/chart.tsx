import equal from "fast-deep-equal";
import { createChart, type IPaneApi, type Time } from "lightweight-charts";
import {
	forwardRef,
	useImperativeHandle,
	useLayoutEffect,
	useRef,
	useState,
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
		const paneStretchFactorsRef = useRef(paneStretchFactors);

		const [lifecycle] = useState((): ChartLifecycle => {
			let chartApi: ReturnType<typeof createChart> | null = null;
			let isRemoved = false;
			let generation = 0;
			// The factor applied to each pane, by the pane's API object, which the
			// library keeps for the life of the pane.
			let appliedFactors = new WeakMap<IPaneApi<Time>, number>();
			return {
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
				// Applied to a pane when it is new, such as one recreated by its
				// series remounting, or when its factor changes; otherwise the pane
				// keeps the height the user dragged it to. Setting a factor redraws
				// the whole chart, so unchanged ones are skipped.
				applyPaneStretchFactors(factors) {
					if (chartApi === null || isRemoved) return;
					const panes = chartApi.panes();
					const changes = factors.flatMap((factor, index) => {
						const pane = panes[index];
						return pane && appliedFactors.get(pane) !== factor
							? [{ factor, pane }]
							: [];
					});
					for (const { factor, pane } of changes) {
						pane.setStretchFactor(factor);
						appliedFactors.set(pane, factor);
					}
				},
				forgetPaneStretchFactors() {
					appliedFactors = new WeakMap();
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
		});

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
			paneStretchFactorsRef.current = paneStretchFactors;
			lifecycle.applyPaneStretchFactors(paneStretchFactors);
		});

		// A double click on the chart returns the panes to the heights they are
		// given, undoing the heights the user dragged them to.
		useLayoutEffect(() => {
			const chart = lifecycle.api();
			const resetPaneHeights = () => {
				lifecycle.forgetPaneStretchFactors();
				lifecycle.applyPaneStretchFactors(paneStretchFactorsRef.current);
			};
			chart.subscribeDblClick(resetPaneHeights);
			return () => {
				if (!lifecycle.isRemoved) chart.unsubscribeDblClick(resetPaneHeights);
			};
		}, [lifecycle]);

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
