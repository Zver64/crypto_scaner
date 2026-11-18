import type { CandleHistoryLoadInput } from "@/api/generated/models";
import { chartIntervals } from "@/features/candle-chart/intervals";
import { historyLoadSchema } from "@/features/command-settings/schemas";
import type { HistoryLoadValues } from "@/features/command-settings/types";
import { type ApiError, describeApiError } from "@/utils/api-error";

export function historyLoadErrorMessage(error: ApiError | null): string {
	return describeApiError(error, {
		fallback: "The history load could not be started.",
		forbidden: "Only the administrator can load history.",
		messages: {
			history_load_running:
				"A history load is already running. Wait until it finishes.",
		},
		server: ["invalid_argument", "symbol_not_found"],
	});
}

// Builds the start request, with the intervals in chart order rather than in
// the order chosen; returns undefined while no coin or interval is chosen or
// the depth is outside its limits.
export function historyLoadRequest(
	values: HistoryLoadValues,
): CandleHistoryLoadInput | undefined {
	const parsed = historyLoadSchema.safeParse(values);
	if (!parsed.success) return undefined;
	const { coins, depth, intervals: chosen } = parsed.data;
	const intervals = chartIntervals.filter((value) => chosen.includes(value));
	return intervals.length === 0
		? undefined
		: { depth, intervals, symbols: coins };
}
