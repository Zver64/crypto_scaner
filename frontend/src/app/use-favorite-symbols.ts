import {
	type listStrategySymbolsResponseSuccess,
	useListStrategySymbols,
} from "@/api/generated/api";

const selectSymbols = (response: listStrategySymbolsResponseSuccess) =>
	response.data.items;

// The administrator's favorites: the coins strategies read through of(), and
// the coins offered for backtests and history loads.
export function useFavoriteSymbols() {
	return useListStrategySymbols({
		query: { retry: false, select: selectSymbols },
	});
}
