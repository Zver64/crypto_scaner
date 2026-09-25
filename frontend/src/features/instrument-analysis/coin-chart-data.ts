import type { CandleInterval } from "@/api/generated/models";
import { LiveCandlesClient } from "@/api/live-candles";
import { getTelegramInitData } from "@/app/telegram";
import type {
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/components/price-history-chart";
import { rsiIndicators } from "@/features/instrument-analysis/chart-page";
import {
	chartIntervals,
	createLiveStore,
	type LiveCandlesState,
} from "@/features/instrument-analysis/live-candle-store";

const initialLimit = 200;
const maxLimit = 5000;

// The backend owns each chart range: it sends a full snapshot whenever closed
// history or the range changes and a tail update for the current candle.
export function createCoinChartData(symbol: string): PriceHistorySource {
	const upper = symbol.toUpperCase();
	const live = createLiveStore(upper);
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
	let connection: LiveCandlesClient | undefined;
	let unsubscribeLive: (() => void) | undefined;
	const subscriptions = () =>
		chartIntervals.map((interval) => ({
			symbol: upper,
			interval,
			limit: intervals.get(interval)?.limit ?? initialLimit,
			indicators: rsiIndicators,
		}));
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
			indicator: state.chart?.indicators[0]?.series[0]?.points ?? [],
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
				getInitData: getTelegramInitData,
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
		loadOlder(interval) {
			const selected = chartIntervals.find((item) => item === interval);
			const current = selected && intervals.get(selected);
			if (
				!selected ||
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
	indicator: [],
	connection: "connecting",
	freshness: "waiting",
	isLoading: true,
	isLoadingMore: false,
	hasMore: false,
};
