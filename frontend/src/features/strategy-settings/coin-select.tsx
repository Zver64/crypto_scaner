import { Select } from "@mantine/core";

import { useInstrumentSymbols } from "@/app/use-instrument-symbols";

export interface CoinSelectProps {
	coin: string;
	disabled: boolean;
	onChange(symbol: string): void;
}

// Chooses the coin of of among the active coins, the coins strategies can
// read. A stored coin that left them stays shown, so the expression is kept.
export function CoinSelect({ coin, disabled, onChange }: CoinSelectProps) {
	const symbols = useInstrumentSymbols();
	const data = symbols.data ?? [];
	return (
		<Select
			aria-label="Coin"
			clearable
			data={coin === "" || data.includes(coin) ? data : [coin, ...data]}
			disabled={disabled}
			error={symbols.isError ? "Coins could not be loaded." : undefined}
			nothingFoundMessage="No coins found"
			// Clearing leaves the coin to be chosen again.
			onChange={(symbol) => onChange(symbol ?? "")}
			placeholder="Coin"
			searchable
			size="sm"
			value={coin === "" ? null : coin}
		/>
	);
}
