import type { ErrorType } from "@/api/fetch";
import type { APIErrorCode, ErrorResponse } from "@/api/generated/models";

// The message of a failed request: the server's own message for the codes in
// `server` (validation and conflicts), an override from `messages`, `forbidden`
// for both access codes, or the fallback. An expired session has one message
// everywhere unless overridden.
export function describeApiError(
	error: ErrorType<ErrorResponse> | null,
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
