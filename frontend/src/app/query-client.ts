import { notifications } from "@mantine/notifications";
import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import {
	type ApiError,
	type ApiErrorCode,
	apiErrorCode,
	apiErrorMessage,
} from "@/features/analysis/api-error";
import { handleUserAccessError } from "@/features/favorites/access-block";
import {
	removeUserScopeQueries,
	userQueryScope,
} from "@/features/favorites/user-query-scope";

export type QueryErrorNotification = {
	id: string;
	title: string;
	// An error the screen shows in place instead of notifying.
	ignoredCode?: ApiErrorCode;
	// Only a failed first load notifies; a failed refetch keeps the data quietly.
	initialLoadOnly?: boolean;
};

// Type aliases, not interfaces: without an index signature a misspelled meta
// key fails typecheck.
type AppQueryMeta = {
	errorNotification?: QueryErrorNotification;
	// A 401 or 403 blocks the Telegram user's favorites access for a while.
	userScoped?: boolean;
};

type AppMutationMeta = {
	userScoped?: boolean;
};

declare module "@tanstack/react-query" {
	interface Register {
		queryMeta: AppQueryMeta;
		mutationMeta: AppMutationMeta;
	}
}

export function showQueryErrorNotification(
	{ id, title }: QueryErrorNotification,
	error: ApiError,
) {
	notifications.show({
		id,
		autoClose: 5000,
		color: "red",
		message: apiErrorMessage(error),
		title,
	});
}

// Query errors are reported here from each query's meta, once per failed
// fetch, instead of in screen effects.
export function createQueryClient() {
	const queryClient: QueryClient = new QueryClient({
		mutationCache: new MutationCache({
			onError: (error, _variables, _result, mutation) => {
				if (mutation.meta?.userScoped) {
					handleUserAccessError(queryClient, error);
				}
			},
		}),
		queryCache: new QueryCache({
			onError: (error, query) => {
				const { errorNotification, userScoped } = query.meta ?? {};
				if (userScoped) handleUserAccessError(queryClient, error);
				if (
					!errorNotification ||
					(errorNotification.initialLoadOnly &&
						query.state.data !== undefined) ||
					(errorNotification.ignoredCode !== undefined &&
						apiErrorCode(error) === errorNotification.ignoredCode)
				) {
					return;
				}
				showQueryErrorNotification(errorNotification, error);
			},
		}),
	});
	// The cache keeps only the current Telegram user's queries: adding one
	// drops those of any other user.
	queryClient.getQueryCache().subscribe((event) => {
		if (event.type !== "added") return;
		const scope = userQueryScope(event.query.queryKey);
		if (scope === undefined) return;
		removeUserScopeQueries(queryClient, (queryScope) => queryScope !== scope);
	});
	return queryClient;
}
