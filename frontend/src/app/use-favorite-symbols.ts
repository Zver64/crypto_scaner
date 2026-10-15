import { useFavorites } from "@/features/favorites/favorites-context";

// The current user's active favorites, ascending: the administrator's on
// the strategy, backtest, and command pages.
export function useFavoriteSymbols() {
	const { favorites, isError, isLoading } = useFavorites();
	const data = [...favorites.values()]
		.filter(({ active }) => active)
		.map(({ symbol }) => symbol)
		.sort();
	return { data, isError, isPending: isLoading };
}
