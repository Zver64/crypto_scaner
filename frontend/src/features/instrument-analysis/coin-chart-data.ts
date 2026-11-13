import { getCurrentUser } from "@/api/generated/api";
import type { CandleInterval } from "@/api/generated/models";
import { LiveCandlesClient } from "@/api/live-candles";
import { getSessionToken, invalidateSessionToken } from "@/api/session";
import type {
	PriceComparison,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart";
import { emptyChartLine } from "@/features/candle-chart/constants";
import type { ChartLineData } from "@/features/candle-chart/types";
import {
	createChartSeriesData,
	createComparisonLine,
} from "@/features/candle-chart/utils";
import {
	type ChartCatalogs,
	chartIntervals,
	createLiveStore,
	type LiveCandlesState,
} from "@/features/instrument-analysis/live-candle-store";

// The chart source also reports the current price for blocks outside the
// chart: the close of the live candle on screen.
export interface CoinChartData extends PriceHistorySource {
	getCurrentPrice(): number | undefined;
	// Bitcoin's price to compare the coin with; absent on Bitcoin's own chart.
	comparison?: PriceComparison;
}

const initialLimit = 200;
const maxLimit = 2000;
const comparisonSymbol = "BTCUSDT";
const comparisonLabel = "BTC";
// The comparison is subscribed without indicators.
const comparisonCatalogs: ChartCatalogs = {
	"1h": [],
	"1d": [],
	"1w": [],
	"1M": [],
};

// The backend owns each chart range: it sends a full snapshot whenever closed
// history or the range changes and a tail update for the current candle. Only
// the interval on screen is subscribed; others keep their last chart as stale.
// While the comparison is enabled, its candles are subscribed on the same
// connection with the interval and range of the coin.
export function createCoinChartData(
	symbol: string,
	catalogs: ChartCatalogs,
): CoinChartData {
	const upper = symbol.toUpperCase();
	let connection: LiveCandlesClient | undefined;
	const live = createLiveStore(upper, catalogs, (interval) =>
		connection?.resync(upper, interval),
	);
	const compared =
		upper === comparisonSymbol
			? undefined
			: createLiveStore(comparisonSymbol, comparisonCatalogs, (interval) =>
					connection?.resync(comparisonSymbol, interval),
				);
	let comparing = false;
	// The line of the last coin slots and comparison chart it was read for.
	let comparisonLine:
		| { slots: unknown; chart: unknown; line: ChartLineData }
		| undefined;
	const listeners = new Set<() => void>();
	const notify = () => {
		for (const listener of listeners) listener();
	};
	const subscribe = (listener: () => void) => {
		listeners.add(listener);
		return () => {
			listeners.delete(listener);
		};
	};
	const setConnection = (next: LiveCandlesState["connection"]) => {
		live.connection(next);
		compared?.connection(next);
	};
	type IntervalState = {
		limit: number;
		loadingMore: boolean;
		input?: LiveCandlesState;
		snapshot?: PriceHistorySnapshot;
	};
	const intervals = new Map<CandleInterval, IntervalState>(
		chartIntervals.map((interval) => [
			interval,
			{ limit: initialLimit, loadingMore: false },
		]),
	);
	let unsubscribeLive: (() => void) | undefined;
	let shown: CandleInterval | undefined;
	const subscriptions = () => {
		if (!shown) return [];
		const limit = intervals.get(shown)?.limit ?? initialLimit;
		return [
			{
				symbol: upper,
				interval: shown,
				limit,
				indicators: catalogs[shown].map(({ parameters, type }) => ({
					parameters,
					type,
				})),
			},
			...(comparing
				? [{ symbol: comparisonSymbol, interval: shown, limit }]
				: []),
		];
	};
	const recompute = (interval: CandleInterval, force = false) => {
		const current = intervals.get(interval);
		if (!current) return;
		const state = live.getSnapshot(interval);
		if (!force && current.input === state) return;
		// Connection and freshness changes keep the chart, and with it the
		// chart-ready series, so the chart redraws only for new data.
		const series =
			current.snapshot && current.input?.chart === state.chart
				? current.snapshot.series
				: createChartSeriesData(
						state.chart?.candles ?? [],
						state.chart,
						catalogs[interval],
						interval,
					);
		current.input = state;
		const closed = state.chart?.candles.length ?? 0;
		if (
			current.loadingMore &&
			(state.error ||
				(state.chart && (!state.chart.has_more || closed >= current.limit)))
		)
			current.loadingMore = false;
		current.snapshot = {
			candles: state.chart?.candles ?? [],
			series,
			connection: state.connection,
			freshness: state.freshness,
			error: state.error,
			isLoading: !state.chart && !state.error,
			isLoadingMore: current.loadingMore,
			hasMore: Boolean(state.chart?.has_more) && current.limit < maxLimit,
		};
		notify();
	};
	const comparison: PriceComparison | undefined = compared && {
		label: comparisonLabel,
		getLine(interval) {
			const selected = chartIntervals.find((item) => item === interval);
			const slots =
				selected && intervals.get(selected)?.snapshot?.series.candles;
			const chart = selected && compared.getSnapshot(selected).chart;
			if (!comparing || !slots || !chart) return emptyChartLine;
			if (comparisonLine?.slots !== slots || comparisonLine.chart !== chart) {
				comparisonLine = {
					slots,
					chart,
					line: createComparisonLine(slots, chart.candles),
				};
			}
			return comparisonLine.line;
		},
		subscribe,
		setEnabled(enabled) {
			if (comparing === enabled) return;
			comparing = enabled;
			connection?.setSubscriptions(subscriptions());
			notify();
		},
	};
	return {
		comparison,
		getSnapshot(interval) {
			const selected = chartIntervals.find((item) => item === interval);
			return (selected && intervals.get(selected)?.snapshot) ?? emptySnapshot;
		},
		getCurrentPrice() {
			return shown && intervals.get(shown)?.snapshot?.candles.at(-1)?.close;
		},
		subscribe,
		start() {
			if (connection) return;
			setConnection("connecting");
			const unsubscribeCoin = live.subscribe(() => {
				for (const interval of chartIntervals) recompute(interval);
			});
			const unsubscribeCompared = compared?.subscribe(notify);
			unsubscribeLive = () => {
				unsubscribeCoin();
				unsubscribeCompared?.();
			};
			connection = new LiveCandlesClient({
				getToken: getSessionToken,
				invalidateToken: invalidateSessionToken,
				checkSession: getCurrentUser,
				onConnectionChange: setConnection,
				onMessage(message) {
					live.message(message);
					compared?.message(message);
				},
			});
			connection.setSubscriptions(subscriptions());
			connection.connect();
		},
		stop() {
			connection?.disconnect();
			connection = undefined;
			setConnection("disconnected");
			unsubscribeLive?.();
			unsubscribeLive = undefined;
			for (const [interval, current] of intervals) {
				current.loadingMore = false;
				recompute(interval, true);
			}
		},
		show(interval) {
			const selected = chartIntervals.find((item) => item === interval);
			if (!selected || selected === shown) return;
			shown = selected;
			connection?.setSubscriptions(subscriptions());
		},
		loadOlder(interval) {
			const selected = chartIntervals.find((item) => item === interval);
			const current = selected && intervals.get(selected);
			if (
				!selected ||
				selected !== shown ||
				!current ||
				!current.snapshot?.hasMore ||
				current.loadingMore ||
				!connection
			)
				return;
			current.limit = Math.min(maxLimit, current.limit + initialLimit);
			current.loadingMore = true;
			// Subscribing with a larger range makes the backend recalculate the
			// indicators over the whole extended range and send a new snapshot.
			connection.setSubscriptions(subscriptions());
			recompute(selected, true);
		},
	};
}

const emptySnapshot: PriceHistorySnapshot = {
	candles: [],
	series: createChartSeriesData([], undefined, [], "1h"),
	connection: "connecting",
	freshness: "waiting",
	isLoading: true,
	isLoadingMore: false,
	hasMore: false,
};
