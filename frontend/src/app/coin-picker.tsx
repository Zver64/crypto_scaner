import { Checkbox, Stack } from "@mantine/core";
import type { ReactNode } from "react";
import {
	type listInstrumentsResponseSuccess,
	useListInstruments,
} from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useFavoriteSymbols } from "@/app/use-favorite-symbols";

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

const selectSymbols = (response: listInstrumentsResponseSuccess) =>
	response.data.items.map(({ symbol }) => symbol).sort();

// A coin select over the administrator's favorites, or every active coin once
// "Favorites only" is cleared. The children render the select itself.
export function CoinPicker({
	allCoins,
	children,
	disabled = false,
	onAllCoinsChange,
}: CoinPickerProps) {
	const permission = useBusinessRequestPermission();
	const favorites = useFavoriteSymbols();
	const instruments = useListInstruments({
		query: {
			enabled: permission.allowed && allCoins,
			retry: false,
			select: selectSymbols,
		},
	});
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
