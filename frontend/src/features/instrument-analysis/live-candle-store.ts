import type {
	CandleInterval,
	ChartPageResponse,
	LiveCandleServerMessage,
} from "@/api/generated/models";
import type {
	ChartConnection,
	ChartFreshness,
} from "@/components/price-history-chart/types";
import {
	mergeChartTail,
	validateChartPage,
} from "@/features/instrument-analysis/chart-page";
import { coinChartIntervals } from "@/features/instrument-analysis/coin-chart-presentation";

export interface LiveCandlesState {
	chart?: ChartPageResponse;
	version?: number;
	connection: ChartConnection;
	freshness: ChartFreshness;
	error?: string;
}

export const chartIntervals: readonly CandleInterval[] = coinChartIntervals.map(
	(item) => item.value,
);
const initialState: LiveCandlesState = {
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
						version: next === "connected" ? undefined : state.version,
						error: next === "disconnected" ? state.error : undefined,
						freshness:
							next === "connected"
								? state.freshness
								: state.chart
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
				return [
					key,
					{
						...(states[key] ?? { ...initialState, connection }),
						error: message.message ?? "Live chart is unavailable",
						freshness: "stale",
					},
				];
			}),
		]);
	}
	if (!message.symbol || !message.interval) return states;
	const key = `${message.symbol}:${message.interval}`;
	const previous = states[key] ?? { ...initialState, connection };
	if (message.type === "snapshot" || message.type === "update") {
		const { chart: received, version } = message;
		if (!received || version === undefined) return states;
		const accepted =
			message.type === "snapshot"
				? previous.version === undefined || version > previous.version
				: previous.chart !== undefined &&
					previous.version !== undefined &&
					version === previous.version + 1;
		if (!accepted) return states;
		let chart: ChartPageResponse;
		try {
			chart = validateChartPage(
				message.type === "update" && previous.chart
					? mergeChartTail(previous.chart, received)
					: received,
				symbol,
				message.interval,
			);
		} catch {
			return states;
		}
		return {
			...states,
			[key]: {
				...previous,
				chart,
				version,
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
