import { MultiSelect, type MultiSelectProps } from "@mantine/core";
import { useState } from "react";
import { CoinPicker } from "@/app/coin-picker";
import { maxHistoryLoadCoins } from "@/features/command-settings/constants";

// Takes the input props of the coins field as the form's getInputProps
// returns them, typed like Mantine's own MultiSelect.
interface HistoryLoadCoinsSelectProps
	extends Pick<
		MultiSelectProps,
		"error" | "onBlur" | "onChange" | "onFocus" | "value"
	> {
	disabled: boolean;
}

export function HistoryLoadCoinsSelect({
	disabled,
	error,
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
					error={select.error ?? error}
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
