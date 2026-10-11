import { Checkbox, MultiSelect, Stack } from "@mantine/core";
import { CoinPicker } from "@/app/coin-picker";
import { useFavoriteSymbols } from "@/app/use-favorite-symbols";

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
	const selectedAll =
		(favorites.data?.length ?? 0) > 0 &&
		symbols.length === favorites.data?.length &&
		favorites.data.every((symbol) => symbols.includes(symbol));
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
			<Checkbox
				checked={selectedAll}
				disabled={disabled || !favorites.data?.length}
				label="Select all favorites"
				onChange={(event) =>
					onChange(
						event.currentTarget.checked ? [...(favorites.data ?? [])] : [],
						allCoins,
					)
				}
			/>
		</Stack>
	);
}
