import type { User } from "@/api/generated/models";

export function formatUserName(user: User): string {
	return user.display_name || (user.username ? `@${user.username}` : "User");
}

export function deleteUserErrorMessage(error: unknown): string {
	const code = (error as { info?: { error?: { code?: unknown } } }).info?.error
		?.code;
	switch (code) {
		case "user_not_found":
			return "This user no longer exists.";
		case "administrator_protected":
			return "The administrator cannot be deleted.";
		case "administrator_required":
		case "access_denied":
			return "Only the administrator can delete users.";
		case "unauthenticated":
			return "Telegram authorization has expired. Reopen the Mini App.";
		default:
			return "The user could not be deleted.";
	}
}

export function updateUserErrorMessage(error: unknown): string {
	const code = (error as { info?: { error?: { code?: unknown } } }).info?.error
		?.code;
	switch (code) {
		case "user_not_found":
			return "This user no longer exists.";
		case "administrator_protected":
			return "The administrator always receives strategy alerts.";
		case "administrator_required":
		case "access_denied":
			return "Only the administrator can change users.";
		case "unauthenticated":
			return "Telegram authorization has expired. Reopen the Mini App.";
		default:
			return "The user could not be changed.";
	}
}
