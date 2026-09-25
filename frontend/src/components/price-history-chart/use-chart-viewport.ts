import { useCallback, useLayoutEffect, useRef, useState } from "react";
import type {
	ChartCanvasHandle,
	ChartLogicalRange,
} from "@/components/lightweight-chart";

interface ViewportOptions {
	barWidth: number;
	data: readonly { time: number }[];
	hasMore: boolean;
	isLoadingMore: boolean;
	minVisibleBars: number;
	onLoadOlder(): void;
	threshold: number;
}

// Keep a logical viewport steady while data is prepended or appended. This is
// chart behavior, independent of the instrument and its candle interval.
export function useChartViewport({
	barWidth,
	data,
	hasMore,
	isLoadingMore,
	minVisibleBars,
	onLoadOlder,
	threshold,
}: ViewportOptions) {
	const chartRef = useRef<ChartCanvasHandle>(null);
	const [visibleRange, setVisibleRange] = useState<ChartLogicalRange | null>(
		null,
	);
	const chartInstanceRef = useRef<{
		handle: ChartCanvasHandle;
		generation: number;
	} | null>(null);
	const previousLengthRef = useRef(0);
	const previousFirstTimeRef = useRef<number | undefined>(undefined);
	const previousRangeRef = useRef<ChartLogicalRange | null>(null);

	const onBeforeDataChange = useCallback(() => {
		const chartHandle = chartRef.current;
		previousRangeRef.current =
			chartHandle === null || previousLengthRef.current === 0
				? null
				: chartHandle.getVisibleLogicalRange();
	}, []);
	const onVisibleLogicalRangeChange = useCallback(
		(range: ChartLogicalRange | null) => {
			setVisibleRange(range);
			if (
				range !== null &&
				range.from < threshold &&
				hasMore &&
				!isLoadingMore
			) {
				onLoadOlder();
			}
		},
		[hasMore, isLoadingMore, onLoadOlder, threshold],
	);

	useLayoutEffect(() => {
		const chartHandle = chartRef.current;
		if (chartHandle === null || data.length === 0) {
			if (data.length === 0) {
				previousLengthRef.current = 0;
				previousFirstTimeRef.current = undefined;
				previousRangeRef.current = null;
			}
			return;
		}
		const generation = chartHandle.generation();
		if (
			chartInstanceRef.current?.handle !== chartHandle ||
			chartInstanceRef.current.generation !== generation
		) {
			chartInstanceRef.current = { handle: chartHandle, generation };
			previousLengthRef.current = 0;
			previousFirstTimeRef.current = undefined;
			previousRangeRef.current = null;
		}
		const previousLength = previousLengthRef.current;
		const previousRange = previousRangeRef.current;
		previousRangeRef.current = null;
		if (previousLength === 0) {
			const visibleBars = Math.max(
				minVisibleBars,
				Math.floor(chartHandle.containerWidth() / barWidth),
			);
			chartHandle.setVisibleLogicalRange({
				from: Math.max(0, data.length - visibleBars),
				to: data.length - 1,
			});
		} else if (previousRange && data.length > previousLength) {
			const previousFirst = previousFirstTimeRef.current;
			const prepended =
				previousFirst === undefined
					? 0
					: Math.max(
							0,
							data.findIndex((item) => item.time === previousFirst),
						);
			const appended = Math.max(0, data.length - previousLength - prepended);
			const followingLatest = previousRange.to >= previousLength - 1.5;
			const shift = prepended + (followingLatest ? appended : 0);
			chartHandle.setVisibleLogicalRange({
				from: previousRange.from + shift,
				to: previousRange.to + shift,
			});
		}
		previousLengthRef.current = data.length;
		previousFirstTimeRef.current = data[0]?.time;
	}, [barWidth, data, minVisibleBars]);

	return {
		chartRef,
		onBeforeDataChange,
		onVisibleLogicalRangeChange,
		visibleRange,
	};
}
