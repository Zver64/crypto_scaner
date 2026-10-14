import type { QueryClient } from "@tanstack/react-query";

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

// The Telegram user scope a query key ends with, if it is user-scoped.
export function userQueryScope(
	queryKey: readonly unknown[],
): string | undefined {
	const scope = (queryKey.at(-1) as { userScope?: unknown } | null | undefined)
		?.userScope;
	return typeof scope === "string" ? scope : undefined;
}

export function removeUserScopeQueries(
	queryClient: QueryClient,
	matches: (scope: string) => boolean,
) {
	queryClient.removeQueries({
		predicate: ({ queryKey }) => {
			const scope = userQueryScope(queryKey);
			return scope !== undefined && matches(scope);
		},
	});
}
