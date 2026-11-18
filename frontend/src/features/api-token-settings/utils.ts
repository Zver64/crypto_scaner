import { type ApiError, describeApiError } from "@/utils/api-error";

export function apiTokenErrorMessage(error: ApiError | null): string {
	return describeApiError(error, {
		fallback: "The API token could not be changed.",
		forbidden: "Only the administrator can manage API tokens.",
		messages: {
			api_token_not_found: "This token no longer exists.",
		},
		server: ["invalid_argument"],
	});
}
