import { Stack, Text } from "@mantine/core";
import { NumberInputFieldset } from "@/components/number-input-fieldset";

interface StrategyMarketCapFieldsProps {
	disabled: boolean;
	// Explain why a bound cannot be saved.
	errors: { maximum?: string; minimum?: string };
	maximum: number | string;
	minimum: number | string;
	onMaximumChange(value: number | string): void;
	onMinimumChange(value: number | string): void;
}

// The market cap range, in millions of USD, of the coins the strategy buys.
export function StrategyMarketCapFields({
	disabled,
	errors,
	maximum,
	minimum,
	onMaximumChange,
	onMinimumChange,
}: StrategyMarketCapFieldsProps) {
	const field = {
		allowNegative: false,
		disabled,
		prefix: "$",
		suffix: "M",
		thousandSeparator: ",",
	};
	return (
		<Stack gap={4}>
			<NumberInputFieldset
				inputs={[
					{
						...field,
						error: errors.minimum,
						id: "strategy-min-market-cap",
						label: "Minimum",
						onChange: onMinimumChange,
						value: minimum,
					},
					{
						...field,
						error: errors.maximum,
						id: "strategy-max-market-cap",
						label: "Maximum",
						onChange: onMaximumChange,
						value: maximum,
					},
				]}
				title="Market cap"
			/>
			<Text c="dimmed" size="xs">
				Buys only coins whose current market cap lies in this range; coins
				without a known market cap then buy nothing. Open trades still sell, and
				backtests ignore the range. Leave empty for no limit.
			</Text>
		</Stack>
	);
}
