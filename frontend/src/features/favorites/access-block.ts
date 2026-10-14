import type { QueryClient } from "@tanstack/react-query";
import { useSyncExternalStore } from "react";
import {
	removeUserScopeQueries,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";

// The query cache error handlers write the block outside React, so it lives in
// this module; the timer ends it without an effect.
let blockedScope: string | undefined;
let unblockTimer: ReturnType<typeof setTimeout> | undefined;
const listeners = new Set<() => void>();

function setBlockedScope(scope: string | undefined) {
	blockedScope = scope;
	for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
	listeners.add(listener);
	return () => {
		listeners.delete(listener);
	};
}

function isAccessError(error: unknown): boolean {
	const status = (error as { status?: unknown }).status;
	return status === 401 || status === 403;
}

// A 401 or 403 of a user-scoped request drops the user's cached data and
// pauses the favorites list for 15 seconds; repeated failures while it is
// paused do not extend the pause.
export function handleUserAccessError(
	queryClient: QueryClient,
	error: unknown,
) {
	if (!isAccessError(error)) return;
	const scope = telegramUserScope();
	if (blockedScope !== scope) {
		clearTimeout(unblockTimer);
		setBlockedScope(scope);
		unblockTimer = setTimeout(() => setBlockedScope(undefined), 15_000);
	}
	removeUserScopeQueries(queryClient, (queryScope) => queryScope === scope);
}

export function useUserAccessBlocked(scope: string): boolean {
	return useSyncExternalStore(
		subscribe,
		() => blockedScope === scope,
		() => false,
	);
}
