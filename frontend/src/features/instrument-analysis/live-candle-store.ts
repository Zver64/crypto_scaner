import type {
	CandleInterval,
	ChartIndicatorDefinition,
	ChartPageResponse,
	LiveCandleServerMessage,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import type {
	ChartConnection,
	ChartFreshness,
} from "@/features/candle-chart/types";
import {
	mergeChartTail,
	validateChartPage,
} from "@/features/instrument-analysis/chart-page";

export interface LiveCandlesState {
	chart?: ChartPageResponse;
	version?: number;
	connection: ChartConnection;
	freshness: ChartFreshness;
	error?: string;
	// An update could not be applied, so the chart waits for a new snapshot.
	resyncing?: boolean;
}

export const chartIntervals: readonly CandleInterval[] =
	chartIntervalOptions.map((item) => item.value);
const initialState: LiveCandlesState = {
	connection: "connecting",
	freshness: "waiting",
};

// Each interval has its own backend catalog; a chart validates results against
// the catalog of its interval.
export type ChartCatalogs = Readonly<
	Record<CandleInterval, readonly ChartIndicatorDefinition[]>
>;

// onResync asks the backend for a new snapshot of an interval whose update
// could not be applied; it is called once until a snapshot is accepted.
export function createLiveStore(
	symbol: string,
	catalogs: ChartCatalogs,
	onResync: (interval: CandleInterval) => void = () => {},
) {
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
						resyncing: next === "connected" ? false : state.resyncing,
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
				message.interval ? catalogs[message.interval] : [],
			);
			if (next === states) return;
			const key = `${symbol}:${message.interval}`;
			const resync = next[key]?.resyncing && !states[key]?.resyncing;
			states = next;
			notify();
			if (resync && message.interval) onResync(message.interval);
		},
	};
}

export function applyServerMessage(
	states: Record<string, LiveCandlesState>,
	message: LiveCandleServerMessage,
	symbol: string,
	connection: LiveCandlesState["connection"],
	catalog: readonly ChartIndicatorDefinition[],
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
		// A late or repeated version is ignored.
		if (previous.version !== undefined && version <= previous.version) {
			return states;
		}
		// An update that cannot be applied (a version gap or an invalid result)
		// would freeze the chart: later updates build on it. Mark the chart stale
		// and wait for a snapshot instead. A rejected snapshot does not resync,
		// so an invalid chart does not loop.
		const stale: Record<string, LiveCandlesState> = {
			...states,
			[key]: {
				...previous,
				freshness: "stale",
				resyncing: message.type === "update" || previous.resyncing,
			},
		};
		if (
			message.type === "update" &&
			(previous.chart === undefined ||
				previous.version === undefined ||
				version !== previous.version + 1)
		) {
			return previous.resyncing ? states : stale;
		}
		let chart: ChartPageResponse;
		try {
			chart = validateChartPage(
				message.type === "update" && previous.chart
					? mergeChartTail(previous.chart, received)
					: received,
				symbol,
				message.interval,
				catalog,
			);
		} catch {
			return previous.resyncing ? states : stale;
		}
		return {
			...states,
			[key]: {
				...previous,
				chart,
				version,
				freshness: message.freshness ?? "fresh",
				error: undefined,
				resyncing: false,
			},
		};
	}
	// The last chart of an interval no longer on screen stays visible as stale
	// until a new subscription sends a fresh snapshot.
	if (message.type === "unsubscribed") {
		return { ...states, [key]: { ...previous, freshness: "stale" } };
	}
	if (message.type === "status" && message.freshness) {
		return { ...states, [key]: { ...previous, freshness: message.freshness } };
	}
	return states;
}
