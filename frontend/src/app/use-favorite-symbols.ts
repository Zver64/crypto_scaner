import { useListStrategySymbols } from "@/api/generated/api";

// The administrator's favorites: the coins strategies read through of(), and
// the coins offered for backtests and history loads.
export function useFavoriteSymbols() {
	return useListStrategySymbols({
		query: { retry: false, select: (response) => response.data.items },
	});
}
