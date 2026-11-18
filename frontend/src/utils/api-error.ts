import type { ErrorType } from "@/api/fetch";
import type { APIErrorCode, ErrorResponse } from "@/api/generated/models";

// A failed request of a generated operation.
export type ApiError = ErrorType<ErrorResponse>;

const unexpectedErrorMessage =
	"An unexpected error occurred. Please try again.";

// Messages of failed queries: the analyses, candles, and the other reads that
// notify through query meta.
const queryErrorMessages = {
	access_denied: "Your Telegram account does not have access to this scanner.",
	insufficient_data: "There is not enough market history for this analysis.",
	invalid_argument: "The request contains unsupported analysis criteria.",
	market_cap_unavailable:
		"Market capitalization is unavailable for this instrument or is still loading. Try again shortly.",
	market_data_unavailable: "Market data is not ready yet. Try again shortly.",
	symbol_not_found: "This instrument is unknown or no longer active.",
	unauthenticated:
		"Your Telegram authorization has expired. Reopen the Mini App and try again.",
} as const satisfies Partial<Record<APIErrorCode, string>>;

export function apiErrorCode(error: ApiError | null): APIErrorCode | undefined {
	return error?.info?.error?.code;
}

// The message of a failed request: the server's own message for the codes in
// `server` (validation and conflicts), an override from `messages`, `forbidden`
// for both access codes, or the fallback. An expired session has one message
// everywhere unless overridden.
export function describeApiError(
	error: ApiError | null,
	{
		fallback,
		forbidden,
		messages = {},
		server = [],
	}: {
		fallback: string;
		forbidden?: string;
		messages?: Partial<Record<APIErrorCode, string>>;
		server?: readonly APIErrorCode[];
	},
): string {
	const info = error?.info?.error;
	if (!info) return fallback;
	if (server.includes(info.code)) return info.message;
	const message = messages[info.code];
	if (message) return message;
	switch (info.code) {
		case "access_denied":
		case "administrator_required":
			return forbidden ?? fallback;
		case "unauthenticated":
			return "Telegram authorization has expired. Reopen the Mini App.";
		default:
			return fallback;
	}
}

// The message of a failed query. An analysis answers 422 when an instrument's
// market capitalization cannot be resolved, whatever its code.
export function describeQueryError(error: ApiError): string {
	if (error.status === 422) return queryErrorMessages.market_cap_unavailable;
	return describeApiError(error, {
		fallback: unexpectedErrorMessage,
		messages: queryErrorMessages,
	});
}

// The error of a query whose response failed validation.
export function unexpectedApiError(): ApiError {
	return new Error(unexpectedErrorMessage);
}
