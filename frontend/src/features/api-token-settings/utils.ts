import type { ErrorType } from "@/api/fetch";
import type { ErrorResponse } from "@/api/generated/models";
import { describeApiError } from "@/utils/api-error";

export function apiTokenErrorMessage(
	error: ErrorType<ErrorResponse> | null,
): string {
	return describeApiError(error, {
		fallback: "The API token could not be changed.",
		forbidden: "Only the administrator can manage API tokens.",
		messages: {
			api_token_not_found: "This token no longer exists.",
		},
		server: ["invalid_argument"],
	});
}
