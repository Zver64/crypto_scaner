import { Select } from "@mantine/core";
import { useListStrategySymbols } from "@/api/generated/api";

export interface CoinSelectProps {
	coin: string;
	disabled: boolean;
	onChange(symbol: string): void;
}

// Chooses the coin of of among the administrator's favorites, the coins
// strategies can read.
// A stored coin that left them stays shown, so the expression is kept.
export function CoinSelect({ coin, disabled, onChange }: CoinSelectProps) {
	const symbols = useListStrategySymbols({
		query: { retry: false, select: (response) => response.data.items },
	});
	const data = symbols.data ?? [];
	return (
		<Select
			aria-label="Coin"
			data={coin === "" || data.includes(coin) ? data : [coin, ...data]}
			disabled={disabled}
			error={
				symbols.isError ? "Favorite coins could not be loaded." : undefined
			}
			nothingFoundMessage="No favorite coins"
			onChange={(symbol) => {
				if (symbol) onChange(symbol);
			}}
			placeholder="Favorite coin"
			searchable
			size="sm"
			value={coin === "" ? null : coin}
		/>
	);
}
