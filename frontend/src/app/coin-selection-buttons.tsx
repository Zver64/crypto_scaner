import { Button, Group } from "@mantine/core";

import { MarketUniverse } from "@/api/generated/models";
import { useFavoriteSymbols } from "@/app/use-favorite-symbols";
import { useInstrumentSymbols } from "@/app/use-instrument-symbols";

interface CoinSelectionButtonsProps {
	disabled: boolean;
	// Receives the chosen coins with the added ones.
	onChange(coins: string[]): void;
	coins: string[];
}

// Adds the active favorites or the coins with a Binance COIN-M perpetual to
// a coin selection.
export function CoinSelectionButtons({
	coins,
	disabled,
	onChange,
}: CoinSelectionButtonsProps) {
	const favorites = useFavoriteSymbols();
	const coinM = useInstrumentSymbols(MarketUniverse.coinm_perpetual);
	const add = (added: string[]) => onChange([...new Set([...coins, ...added])]);
	return (
		<Group gap="xs" grow>
			<Button
				disabled={disabled || favorites.data.length === 0}
				onClick={() => add(favorites.data)}
				type="button"
				variant="default"
			>
				Select all favorites
			</Button>
			<Button
				disabled={disabled || !coinM.data?.length}
				onClick={() => add(coinM.data ?? [])}
				type="button"
				variant="default"
			>
				Select all COIN-M
			</Button>
		</Group>
	);
}
