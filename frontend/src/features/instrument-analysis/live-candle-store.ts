import type {
	CandleInterval,
	LiveCandleServerMessage,
	LiveCandleState,
} from "@/api/generated/models";
import type {
	ChartConnection,
	ChartFreshness,
} from "@/components/price-history-chart/types";
import { validateCandles } from "@/features/instrument-analysis/candle-page";
import { coinChartIntervals } from "@/features/instrument-analysis/coin-chart-presentation";
import { applyLiveCandleUpdate } from "@/features/instrument-analysis/live-candle-merge";

export interface LiveCandlesState {
	candles: readonly LiveCandleState[];
	connection: ChartConnection;
	freshness: ChartFreshness;
	error?: string;
}

export const chartIntervals: readonly CandleInterval[] = coinChartIntervals.map(
	(item) => item.value,
);

const initialState: LiveCandlesState = {
	candles: [],
	connection: "connecting",
	freshness: "waiting",
};

export function createLiveStore(symbol: string) {
	let states: Record<string, LiveCandlesState> = {};
	let fallback = initialState;
	const listeners = new Set<() => void>();
	const notify = () => {
		for (const listener of listeners) listener();
	};
	return {
		getSnapshot(interval: CandleInterval) {
			return states[`${symbol}:${interval}`] ?? fallback;
		},
		subscribe(listener: () => void) {
			listeners.add(listener);
			return () => {
				listeners.delete(listener);
			};
		},
		connection(next: LiveCandlesState["connection"]) {
			fallback = { ...initialState, connection: next };
			states = Object.fromEntries(
				Object.entries(states).map(([key, state]) => [
					key,
					{
						...state,
						connection: next,
						error: next === "disconnected" ? state.error : undefined,
						freshness:
							next === "connected"
								? state.freshness
								: state.candles.length > 0
									? "stale"
									: "waiting",
					},
				]),
			);
			notify();
		},
		message(message: LiveCandleServerMessage) {
			if (message.symbol && message.symbol !== symbol) return;
			const next = applyServerMessage(
				states,
				message,
				symbol,
				fallback.connection,
			);
			if (next === states) return;
			states = next;
			notify();
		},
	};
}

export function applyServerMessage(
	states: Record<string, LiveCandlesState>,
	message: LiveCandleServerMessage,
	symbol: string,
	connection: LiveCandlesState["connection"],
): Record<string, LiveCandlesState> {
	if (message.type === "error") {
		const intervals = message.interval ? [message.interval] : chartIntervals;
		return Object.fromEntries([
			...Object.entries(states),
			...intervals.map((interval) => {
				const key = `${symbol}:${interval}`;
				const previous = states[key] ?? { ...initialState, connection };
				return [
					key,
					{
						...previous,
						error: message.message ?? "Live candles are unavailable",
						freshness: "stale",
					},
				];
			}),
		]);
	}
	if (!message.symbol || !message.interval) return states;
	const key = `${message.symbol}:${message.interval}`;
	const previous = states[key] ?? { ...initialState, connection };
	if (message.type === "snapshot" && message.candles) {
		return {
			...states,
			[key]: {
				...previous,
				candles: validLiveCandles(message.candles)
					? message.candles
					: previous.candles,
				freshness: message.freshness ?? previous.freshness,
				error: undefined,
			},
		};
	}
	if (message.type === "update" && message.candle) {
		if (!validLiveCandles([message.candle])) return states;
		return {
			...states,
			[key]: {
				...previous,
				candles: applyLiveCandleUpdate(previous.candles, message.candle),
				freshness: message.freshness ?? "fresh",
				error: undefined,
			},
		};
	}
	if (message.type === "status" && message.freshness) {
		return { ...states, [key]: { ...previous, freshness: message.freshness } };
	}
	return states;
}

function validLiveCandles(candles: readonly LiveCandleState[]) {
	try {
		validateCandles(
			[...candles]
				.map((state) => state.candle)
				.sort(
					(left, right) =>
						Date.parse(left.open_time) - Date.parse(right.open_time),
				),
		);
		return candles.every((state) => typeof state.final === "boolean");
	} catch {
		return false;
	}
}
