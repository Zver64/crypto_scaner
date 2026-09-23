import { getTelegramInitData } from "@/app/telegram";

const unknownUserScope = "telegram-user:unknown";

export function telegramUserScope(initData = getTelegramInitData()): string {
	if (!initData) return unknownUserScope;
	try {
		const rawUser = new URLSearchParams(initData).get("user");
		if (!rawUser) return unknownUserScope;
		const user = JSON.parse(rawUser) as { id?: unknown };
		return typeof user.id === "number" && Number.isSafeInteger(user.id)
			? `telegram-user:${user.id}`
			: unknownUserScope;
	} catch {
		return unknownUserScope;
	}
}

export function scopedUserQueryKey(
	generatedKey: readonly unknown[],
	scope = telegramUserScope(),
) {
	return [...generatedKey, { userScope: scope }] as const;
}
