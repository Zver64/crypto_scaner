import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { handleUserAccessError } from "@/features/favorites/access-block";
import {
	removeUserScopeQueries,
	userQueryScope,
} from "@/features/favorites/user-query-scope";
import { apiErrorCode } from "@/utils/api-error";
import {
	type QueryErrorNotification,
	showQueryErrorNotification,
} from "@/utils/query-error-notification";

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
