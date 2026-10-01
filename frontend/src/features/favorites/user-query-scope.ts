import { getTelegramInitData, telegramUserID } from "@/app/telegram";

const unknownUserScope = "telegram-user:unknown";

export function telegramUserScope(initData = getTelegramInitData()): string {
	const userID = telegramUserID(initData);
	return userID === undefined ? unknownUserScope : `telegram-user:${userID}`;
}

export function scopedUserQueryKey(
	generatedKey: readonly unknown[],
	scope = telegramUserScope(),
) {
	return [...generatedKey, { userScope: scope }] as const;
}
