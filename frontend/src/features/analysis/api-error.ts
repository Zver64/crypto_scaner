import type {
	AnalyzeInstrumentQueryError,
	AnalyzeMarketQueryError,
	GetInstrumentChartInfiniteQueryError,
	ListInstrumentCandlesInfiniteQueryError,
} from "@/api/generated/api";
import type { ErrorResponse } from "@/api/generated/models";

export type ApiError =
	| AnalyzeInstrumentQueryError
	| AnalyzeMarketQueryError
	| GetInstrumentChartInfiniteQueryError
	| ListInstrumentCandlesInfiniteQueryError;

export type ApiErrorCode =
	| "access_denied"
	| "insufficient_data"
	| "invalid_argument"
	| "market_cap_unavailable"
	| "market_data_unavailable"
	| "symbol_not_found"
	| "unauthenticated"
	| "unexpected_error";

const messages: Record<ApiErrorCode, string> = {
	access_denied: "Your Telegram account does not have access to this scanner.",
	insufficient_data: "There is not enough market history for this analysis.",
	invalid_argument: "The request contains unsupported analysis criteria.",
	market_cap_unavailable:
		"Market capitalization is unavailable for this instrument or is still loading. Try again shortly.",
	market_data_unavailable: "Market data is not ready yet. Try again shortly.",
	symbol_not_found: "This instrument is unknown or no longer active.",
	unauthenticated:
		"Your Telegram authorization has expired. Reopen the Mini App and try again.",
	unexpected_error: "An unexpected error occurred. Please try again.",
};

export function apiErrorCode(error: ApiError): ApiErrorCode {
	if (error.status === 422) {
		return "market_cap_unavailable";
	}
	return canonicalCode(error.info?.error.code);
}

export function apiErrorMessage(error: ApiError): string {
	return messages[apiErrorCode(error)];
}

export function unexpectedApiError(): ApiError {
	return new Error(messages.unexpected_error);
}

function canonicalCode(
	value: ErrorResponse["error"]["code"] | undefined,
): ApiErrorCode {
	switch (value) {
		case "access_denied":
		case "insufficient_data":
		case "invalid_argument":
		case "market_cap_unavailable":
		case "market_data_unavailable":
		case "symbol_not_found":
		case "unauthenticated":
			return value;
		default:
			return "unexpected_error";
	}
}
