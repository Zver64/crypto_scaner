import { Select } from "@mantine/core";
import { CoinPicker } from "@/app/coin-picker";

interface CoinSelectProps {
	allCoins: boolean;
	onChange(symbol: string | undefined, allCoins: boolean): void;
	symbol: string | undefined;
}

export function CoinSelect({ allCoins, onChange, symbol }: CoinSelectProps) {
	return (
		<CoinPicker
			allCoins={allCoins}
			onAllCoinsChange={(nextAllCoins, isFavorite) =>
				onChange(
					nextAllCoins || (symbol && isFavorite(symbol)) ? symbol : undefined,
					nextAllCoins,
				)
			}
		>
			{(select) => (
				<Select
					{...select}
					allowDeselect={false}
					label="Coin"
					onChange={(value) => onChange(value ?? undefined, allCoins)}
					placeholder="Choose a coin"
					searchable
					value={symbol ?? null}
				/>
			)}
		</CoinPicker>
	);
}
