import { Checkbox, Stack } from "@mantine/core";
import type { ReactNode } from "react";

import { useFavoriteSymbols } from "@/app/use-favorite-symbols";
import { useInstrumentSymbols } from "@/app/use-instrument-symbols";

interface CoinSelectProps {
	data: string[];
	disabled: boolean;
	error: string | undefined;
	nothingFoundMessage: string;
}

interface CoinPickerProps {
	allCoins: boolean;
	children(select: CoinSelectProps): ReactNode;
	disabled?: boolean;
	onAllCoinsChange(
		allCoins: boolean,
		isFavorite: (coin: string) => boolean,
	): void;
}

// A coin select over the user's active favorites, or every active coin once
// "Favorites only" is cleared. The children render the select itself.
export function CoinPicker({
	allCoins,
	children,
	disabled = false,
	onAllCoinsChange,
}: CoinPickerProps) {
	const favorites = useFavoriteSymbols();
	const instruments = useInstrumentSymbols(undefined, allCoins);
	const coins = allCoins ? instruments : favorites;
	return (
		<Stack gap="xs">
			{children({
				data: coins.data ?? [],
				disabled: disabled || coins.isPending,
				error: coins.isError ? "Coins could not be loaded." : undefined,
				nothingFoundMessage: allCoins ? "No coins found" : "No favorite coins",
			})}
			<Checkbox
				checked={!allCoins}
				disabled={disabled || favorites.isPending}
				label="Favorites only"
				onChange={(event) =>
					onAllCoinsChange(
						!event.currentTarget.checked,
						(coin) => favorites.data?.includes(coin) ?? false,
					)
				}
			/>
		</Stack>
	);
}
