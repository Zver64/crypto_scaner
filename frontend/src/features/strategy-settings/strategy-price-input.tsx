import { TextInput } from "@mantine/core";

interface StrategyPriceInputProps {
	description: string;
	disabled: boolean;
	label: string;
	onChange(value: string): void;
	value: string;
}

// A take profit or stop loss price expression, checked by the backend when
// the strategy is saved.
export function StrategyPriceInput({
	description,
	disabled,
	label,
	onChange,
	value,
}: StrategyPriceInputProps) {
	return (
		<TextInput
			description={description}
			disabled={disabled}
			label={label}
			maxLength={2000}
			onChange={(event) => onChange(event.currentTarget.value)}
			styles={(theme) => ({ input: { fontFamily: theme.fontFamilyMonospace } })}
			value={value}
		/>
	);
}
