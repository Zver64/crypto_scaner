import type { Candle, LiveCandleState } from "@/api/generated/models";

/** Merges immutable HTTP pages with bounded live snapshots without touching page cursors. */
export function mergeHistoryAndLiveCandles(
	history: readonly Candle[],
	live: readonly LiveCandleState[],
): Candle[] {
	const candles = new Map(history.map((candle) => [candle.open_time, candle]));
	const closedHistory = new Set(candles.keys());
	for (const state of live) {
		// A delayed non-final event must never roll a candle already confirmed by HTTP back.
		if (!state.final && closedHistory.has(state.candle.open_time)) {
			continue;
		}
		candles.set(state.candle.open_time, state.candle);
	}
	return [...candles.values()].sort(
		(left, right) => Date.parse(left.open_time) - Date.parse(right.open_time),
	);
}

export function applyLiveCandleUpdate(
	current: readonly LiveCandleState[],
	incoming: LiveCandleState,
	limit = 16,
): LiveCandleState[] {
	const candles = new Map(
		current.map((state) => [state.candle.open_time, state]),
	);
	const existing = candles.get(incoming.candle.open_time);
	if (!(existing?.final && !incoming.final)) {
		candles.set(incoming.candle.open_time, incoming);
	}
	return [...candles.values()]
		.sort(
			(left, right) =>
				Date.parse(left.candle.open_time) - Date.parse(right.candle.open_time),
		)
		.slice(-limit);
}
