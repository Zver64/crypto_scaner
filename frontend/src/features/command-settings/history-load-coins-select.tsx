import { MultiSelect } from "@mantine/core";
import { useState } from "react";
import { CoinPicker } from "@/app/coin-picker";
import { maxHistoryLoadCoins } from "@/features/command-settings/constants";

interface HistoryLoadCoinsSelectProps {
	disabled: boolean;
	onChange(coins: string[]): void;
	value: string[];
}

export function HistoryLoadCoinsSelect({
	disabled,
	onChange,
	value,
}: HistoryLoadCoinsSelectProps) {
	const [allCoins, setAllCoins] = useState(false);
	return (
		<CoinPicker
			allCoins={allCoins}
			disabled={disabled}
			onAllCoinsChange={(nextAllCoins, isFavorite) => {
				setAllCoins(nextAllCoins);
				if (!nextAllCoins) onChange(value.filter(isFavorite));
			}}
		>
			{(select) => (
				<MultiSelect
					{...select}
					clearable
					description={`Up to ${maxHistoryLoadCoins} coins.`}
					hidePickedOptions
					label="Coins"
					maxValues={maxHistoryLoadCoins}
					onChange={onChange}
					placeholder={value.length === 0 ? "Choose coins" : undefined}
					searchable
					value={value}
				/>
			)}
		</CoinPicker>
	);
}
