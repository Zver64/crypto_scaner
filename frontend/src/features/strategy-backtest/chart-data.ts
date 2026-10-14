import type {
	BacktestChart,
	CandleInterval,
	StrategyBacktest,
} from "@/api/generated/models";
import type {
	ChartIndicatorLayout,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart";
import type { ChartTradeMarkers } from "@/features/candle-chart/types";
import {
	createChartSeriesData,
	createIndicatorLayout,
	createMarkerDataSelector,
} from "@/features/candle-chart/utils";
import { backtestMarkers } from "@/features/strategy-backtest/utils";

// Everything the chart of a completed run draws, created once per run.
export function createBacktestChart(backtest: StrategyBacktest) {
	const indicators: Record<CandleInterval, ChartIndicatorLayout> = {
		"1h": createIndicatorLayout([]),
		"1d": createIndicatorLayout([]),
		"1w": createIndicatorLayout([]),
		"1M": createIndicatorLayout([]),
	};
	for (const { catalog, page } of backtest.charts) {
		indicators[page.interval] = createIndicatorLayout(catalog);
	}
	return {
		indicators,
		intervals: backtest.charts.map(({ page }) => page.interval),
		source: createBacktestChartData(backtest.charts, backtestMarkers(backtest)),
	};
}

// All data belongs to the completed run. Scroll-back reveals older points
// from that snapshot, never fetching or subscribing to changing history.
export function createBacktestChartData(
	charts: readonly BacktestChart[],
	markers?: ChartTradeMarkers,
): PriceHistorySource {
	const listeners = new Set<() => void>();
	const pages = new Map<
		CandleInterval,
		{
			chart: BacktestChart;
			count: number;
			snapshot: PriceHistorySnapshot;
			selectMarkers?: ReturnType<typeof createMarkerDataSelector>;
		}
	>();
	const snapshot = (
		chart: BacktestChart,
		count: number,
		selectMarkers?: ReturnType<typeof createMarkerDataSelector>,
	): PriceHistorySnapshot => {
		const { interval } = chart.page;
		const candles = chart.page.candles.slice(-count);
		const series = createChartSeriesData(
			candles,
			chart.page,
			chart.catalog,
			interval,
		);
		return {
			candles,
			series,
			markers: selectMarkers?.(series.candles),
			connection: "disconnected",
			freshness: "fresh",
			hasMore: candles.length < chart.page.candles.length,
			isLoading: false,
			isLoadingMore: false,
		};
	};
	for (const chart of charts) {
		const selectMarkers =
			markers && createMarkerDataSelector(markers, chart.page.interval);
		pages.set(chart.page.interval, {
			chart,
			count: 200,
			snapshot: snapshot(chart, 200, selectMarkers),
			selectMarkers,
		});
	}
	const empty: PriceHistorySnapshot = {
		candles: [],
		series: createChartSeriesData([], undefined, [], "1h"),
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
			page.snapshot = snapshot(page.chart, page.count, page.selectMarkers);
			for (const listener of listeners) listener();
		},
	};
}
