import type { User } from "@/api/generated/models";
import { type ApiError, describeApiError } from "@/utils/api-error";

export function formatUserName(user: User): string {
	return user.display_name || (user.username ? `@${user.username}` : "User");
}

export function deleteUserErrorMessage(error: ApiError | null): string {
	return describeApiError(error, {
		fallback: "The user could not be deleted.",
		forbidden: "Only the administrator can delete users.",
		messages: {
			administrator_protected: "The administrator cannot be deleted.",
			user_not_found: "This user no longer exists.",
		},
	});
}

export function updateUserErrorMessage(error: ApiError | null): string {
	return describeApiError(error, {
		fallback: "The user could not be changed.",
		forbidden: "Only the administrator can change users.",
		messages: {
			administrator_protected:
				"The administrator always receives strategy alerts.",
			user_not_found: "This user no longer exists.",
		},
	});
}
