import { MultiSelect, type MultiSelectProps } from "@mantine/core";
import { useState } from "react";

import { CoinPicker } from "@/app/coin-picker";
import { maxHistoryLoadCoins } from "@/features/command-settings/constants";

// Takes the coins field's value and handlers, typed like Mantine's own
// MultiSelect.
interface HistoryLoadCoinsSelectProps extends Pick<
	MultiSelectProps,
	"onBlur" | "onChange" | "value"
> {
	disabled: boolean;
}

export function HistoryLoadCoinsSelect({
	disabled,
	onChange,
	value = [],
	...input
}: HistoryLoadCoinsSelectProps) {
	const [allCoins, setAllCoins] = useState(false);
	return (
		<CoinPicker
			allCoins={allCoins}
			disabled={disabled}
			onAllCoinsChange={(nextAllCoins, isFavorite) => {
				setAllCoins(nextAllCoins);
				if (!nextAllCoins) onChange?.(value.filter(isFavorite));
			}}
		>
			{(select) => (
				<MultiSelect
					{...select}
					{...input}
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
