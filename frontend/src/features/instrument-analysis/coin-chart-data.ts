import type {
	CandleInterval,
	ChartIndicatorDefinition,
	ChartPageResponse,
} from "@/api/generated/models";
import { LiveCandlesClient } from "@/api/live-candles";
import { getSessionToken, invalidateSessionToken } from "@/api/session";
import type {
	ChartIndicatorPoints,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart";
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
}

const initialLimit = 200;
const maxLimit = 2000;

// The backend owns each chart range: it sends a full snapshot whenever closed
// history or the range changes and a tail update for the current candle. Only
// the interval on screen is subscribed; others keep their last chart as stale.
export function createCoinChartData(
	symbol: string,
	catalogs: ChartCatalogs,
): CoinChartData {
	const upper = symbol.toUpperCase();
	let connection: LiveCandlesClient | undefined;
	const live = createLiveStore(upper, catalogs, (interval) =>
		connection?.resync(upper, interval),
	);
	const listeners = new Set<() => void>();
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
	const subscriptions = () =>
		shown
			? [
					{
						symbol: upper,
						interval: shown,
						limit: intervals.get(shown)?.limit ?? initialLimit,
						indicators: catalogs[shown].map(({ parameters, type }) => ({
							parameters,
							type,
						})),
					},
				]
			: [];
	const recompute = (interval: CandleInterval, force = false) => {
		const current = intervals.get(interval);
		if (!current) return;
		const state = live.getSnapshot(interval);
		if (!force && current.input === state) return;
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
			indicators: indicatorPoints(state.chart, catalogs[interval]),
			connection: state.connection,
			freshness: state.freshness,
			error: state.error,
			isLoading: !state.chart && !state.error,
			isLoadingMore: current.loadingMore,
			hasMore: Boolean(state.chart?.has_more) && current.limit < maxLimit,
		};
		for (const listener of listeners) listener();
	};
	return {
		getSnapshot(interval) {
			const selected = chartIntervals.find((item) => item === interval);
			return (selected && intervals.get(selected)?.snapshot) ?? emptySnapshot;
		},
		getCurrentPrice() {
			return shown && intervals.get(shown)?.snapshot?.candles.at(-1)?.close;
		},
		subscribe(listener) {
			listeners.add(listener);
			return () => {
				listeners.delete(listener);
			};
		},
		start() {
			if (connection) return;
			live.connection("connecting");
			unsubscribeLive = live.subscribe(() => {
				for (const interval of chartIntervals) recompute(interval);
			});
			connection = new LiveCandlesClient({
				getToken: getSessionToken,
				invalidateToken: invalidateSessionToken,
				onConnectionChange: live.connection,
				onMessage: live.message,
			});
			connection.setSubscriptions(subscriptions());
			connection.connect();
		},
		stop() {
			connection?.disconnect();
			connection = undefined;
			live.connection("disconnected");
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
	indicators: {},
	connection: "connecting",
	freshness: "waiting",
	isLoading: true,
	isLoadingMore: false,
	hasMore: false,
};

// Keys each result's series by catalog id and output name; results arrive in
// catalog order.
function indicatorPoints(
	chart: ChartPageResponse | undefined,
	catalog: readonly ChartIndicatorDefinition[],
): ChartIndicatorPoints {
	if (!chart) return {};
	return Object.fromEntries(
		catalog.map(({ id }, index) => [
			id,
			Object.fromEntries(
				(chart.indicators[index]?.series ?? []).map(({ name, points }) => [
					name,
					points,
				]),
			),
		]),
	);
}
