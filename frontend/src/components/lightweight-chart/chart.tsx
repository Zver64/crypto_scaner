import {
	ColorType,
	createChart,
	type DeepPartial,
	type Time,
	type TimeChartOptions,
} from "lightweight-charts";
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
	ChartCanvasOptions,
	ChartCanvasProps,
} from "@/components/lightweight-chart/types";

function toChartOptions(
	options: ChartCanvasOptions,
): DeepPartial<TimeChartOptions> {
	return {
		layout: {
			background: { color: options.background, type: ColorType.Solid },
			textColor: options.text,
		},
		grid: {
			horzLines: { color: options.grid },
			vertLines: { color: options.grid },
		},
		localization: {
			timeFormatter: (time: Time) =>
				typeof time === "number" ? options.timeFormatter(time) : String(time),
		},
		rightPriceScale: { borderColor: options.grid },
		timeScale: {
			barSpacing: options.barSpacing,
			borderColor: options.grid,
			minBarSpacing: options.minBarSpacing,
			secondsVisible: false,
			timeVisible: options.timeVisible,
		},
	};
}

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
		const optionsRef = useRef(options);

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
							...toChartOptions(optionsRef.current),
							height: container.clientHeight,
							width: container.clientWidth,
						});
					}
					return chartApi;
				},
				generation: () => generation,
				applyPaneStretchFactors(factors) {
					if (chartApi === null || isRemoved) return;
					for (const [index, factor] of factors.entries()) {
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
				generation: () => lifecycle.generation(),
				containerWidth: () => containerRef.current?.clientWidth ?? 0,
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
			optionsRef.current = options;
			lifecycle.api().applyOptions(toChartOptions(options));
		}, [lifecycle, options]);

		useLayoutEffect(() => {
			lifecycle.applyPaneStretchFactors(paneStretchFactors);
		});

		useLayoutEffect(() => {
			const chart = lifecycle.api();
			const timeScale = chart.timeScale();
			const handleRangeChange: Parameters<
				typeof timeScale.subscribeVisibleLogicalRangeChange
			>[0] = (range) => onVisibleLogicalRangeChange?.(range);
			timeScale.subscribeVisibleLogicalRangeChange(handleRangeChange);
			return () => {
				if (!lifecycle.isRemoved) {
					timeScale.unsubscribeVisibleLogicalRangeChange(handleRangeChange);
				}
			};
		}, [lifecycle, onVisibleLogicalRangeChange]);

		return (
			<ChartContext.Provider value={lifecycle}>
				<div ref={containerRef} {...containerProps} />
				{children}
			</ChartContext.Provider>
		);
	},
);
