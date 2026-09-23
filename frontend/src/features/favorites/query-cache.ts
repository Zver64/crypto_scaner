import type { QueryClient } from "@tanstack/react-query";
import {
	getAnalyzeFavoritesQueryKey,
	getListFavoritesQueryKey,
} from "@/api/generated/api";

export function invalidateFavoriteQueries(queryClient: QueryClient) {
	void queryClient.invalidateQueries({
		queryKey: getListFavoritesQueryKey(),
	});
	void queryClient.invalidateQueries({
		queryKey: getAnalyzeFavoritesQueryKey().slice(0, 2),
	});
}
