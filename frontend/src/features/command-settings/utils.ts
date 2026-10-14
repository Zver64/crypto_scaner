import type { ErrorType } from "@/api/fetch";
import type {
	CandleHistoryLoadInput,
	CandleInterval,
	ErrorResponse,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { historyLoadDepth } from "@/features/command-settings/constants";
import type { HistoryLoadValues } from "@/features/command-settings/types";
import { describeApiError } from "@/utils/api-error";
import { formatNumber } from "@/utils/number-format";

export function historyLoadErrorMessage(
	error: ErrorType<ErrorResponse> | null,
): string {
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

export function intervalLabel(interval: CandleInterval): string {
	return (
		chartIntervalOptions.find(({ value }) => value === interval)?.label ??
		interval
	);
}

// Why a depth cannot be loaded; undefined while it is within the limits.
export function historyLoadDepthError(
	depth: number | string,
): string | undefined {
	return typeof depth === "number" &&
		depth >= historyLoadDepth.min &&
		depth <= historyLoadDepth.max
		? undefined
		: `Depth must be from ${formatNumber(historyLoadDepth.min)} to ${formatNumber(historyLoadDepth.max)}`;
}

// Builds the start request, with the intervals in chart order rather than in
// the order chosen; returns undefined while no coin or interval is chosen or
// the depth is outside its limits.
export function historyLoadRequest({
	coins,
	depth,
	intervals: chosenIntervals,
}: HistoryLoadValues): CandleHistoryLoadInput | undefined {
	const intervals = chartIntervalOptions
		.map(({ value }) => value)
		.filter((value) => chosenIntervals.includes(value));
	if (
		coins.length === 0 ||
		intervals.length === 0 ||
		typeof depth !== "number" ||
		historyLoadDepthError(depth) !== undefined
	) {
		return undefined;
	}
	return { depth, intervals, symbols: coins };
}
