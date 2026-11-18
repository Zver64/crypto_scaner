import { MultiSelect, Stack } from "@mantine/core";

import { CoinPicker } from "@/features/coins/coin-picker";
import { CoinSelectionButtons } from "@/features/coins/coin-selection-buttons";
import { useFavoriteSymbols } from "@/features/favorites/use-favorite-symbols";

interface CoinSelectProps {
	allCoins: boolean;
	disabled: boolean;
	onChange(symbols: string[], allCoins: boolean): void;
	symbols: string[];
}

export function CoinSelect({
	allCoins,
	disabled,
	onChange,
	symbols,
}: CoinSelectProps) {
	const favorites = useFavoriteSymbols();
	return (
		<Stack gap="xs">
			<CoinPicker
				allCoins={allCoins}
				disabled={disabled}
				onAllCoinsChange={(nextAllCoins, isFavorite) =>
					onChange(
						nextAllCoins ? symbols : symbols.filter(isFavorite),
						nextAllCoins,
					)
				}
			>
				{(select) => (
					<MultiSelect
						{...select}
						clearable
						hidePickedOptions
						label="Coins"
						onChange={(values) => onChange(values, allCoins)}
						placeholder="Choose coins"
						searchable
						value={symbols}
					/>
				)}
			</CoinPicker>
			{/* Coins beyond the favorites need the select of all coins. */}
			<CoinSelectionButtons
				coins={symbols}
				disabled={disabled}
				onChange={(values) =>
					onChange(
						values,
						allCoins || values.some((coin) => !favorites.data.includes(coin)),
					)
				}
			/>
		</Stack>
	);
}
