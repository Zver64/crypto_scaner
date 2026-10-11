import { Button, MultiSelect, Stack } from "@mantine/core";
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
			<Button
				disabled={disabled || !favorites.data?.length}
				onClick={() =>
					onChange(
						[...new Set([...symbols, ...(favorites.data ?? [])])],
						allCoins,
					)
				}
				type="button"
				variant="default"
			>
				Select all favorites
			</Button>
		</Stack>
	);
}
