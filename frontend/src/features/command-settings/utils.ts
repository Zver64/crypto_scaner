import type { ErrorType } from "@/api/fetch";
import type { CandleInterval, ErrorResponse } from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { describeApiError } from "@/utils/api-error";

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
