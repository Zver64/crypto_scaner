import type { BacktestChart, CandleInterval } from "@/api/generated/models";
import type {
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart";

import { chartIndicatorPoints } from "@/features/candle-chart/utils";

// All data belongs to the completed run. Scroll-back reveals older points
// from that snapshot, never fetching or subscribing to changing history.
export function createBacktestChartData(
	charts: readonly BacktestChart[],
): PriceHistorySource {
	const listeners = new Set<() => void>();
	const pages = new Map<
		CandleInterval,
		{ chart: BacktestChart; count: number; snapshot: PriceHistorySnapshot }
	>();
	const snapshot = (
		chart: BacktestChart,
		count: number,
	): PriceHistorySnapshot => {
		const candles = chart.page.candles.slice(-count);
		const first = candles[0]?.open_time;
		return {
			candles,
			indicators:
				first === undefined
					? {}
					: chartIndicatorPoints(chart.page, chart.catalog, first),
			connection: "disconnected",
			freshness: "fresh",
			hasMore: candles.length < chart.page.candles.length,
			isLoading: false,
			isLoadingMore: false,
		};
	};
	for (const chart of charts)
		pages.set(chart.page.interval, {
			chart,
			count: 200,
			snapshot: snapshot(chart, 200),
		});
	const empty: PriceHistorySnapshot = {
		candles: [],
		indicators: {},
		connection: "disconnected",
		freshness: "fresh",
		hasMore: false,
		isLoading: false,
		isLoadingMore: false,
	};
	return {
		getSnapshot: (interval) => pages.get(interval)?.snapshot ?? empty,
		subscribe(listener) {
			listeners.add(listener);
			return () => {
				listeners.delete(listener);
			};
		},
		start() {},
		stop() {},
		show() {},
		loadOlder(interval) {
			const page = pages.get(interval);
			if (!page?.snapshot.hasMore) return;
			page.count += 200;
			page.snapshot = snapshot(page.chart, page.count);
			for (const listener of listeners) listener();
		},
	};
}
