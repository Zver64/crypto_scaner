import { createContext, useContext } from "react";
import type { Favorite } from "@/api/generated/models";

export interface FavoritesContextValue {
	favorites: ReadonlyMap<string, Favorite>;
	isError: boolean;
	isLoading: boolean;
	isMutating(symbol: string): boolean;
	toggle(symbol: string): void;
}

export const FavoritesContext = createContext<
	FavoritesContextValue | undefined
>(undefined);

export function useFavorites() {
	const value = useContext(FavoritesContext);
	if (!value)
		throw new Error("useFavorites must be used inside FavoritesProvider");
	return value;
}
