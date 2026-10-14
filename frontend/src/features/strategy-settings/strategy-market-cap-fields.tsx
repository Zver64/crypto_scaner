import { Stack, Text } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import { NumberInputFieldset } from "@/components/number-input-fieldset";
import type { StrategyFormValues } from "@/features/strategy-settings/types";
import { useWatchedValue } from "@/features/strategy-settings/use-watched-value";
import {
	marketCapErrors,
	marketCapUsd,
} from "@/features/strategy-settings/utils";

interface StrategyMarketCapFieldsProps {
	disabled: boolean;
	form: UseFormReturnType<StrategyFormValues>;
}

// The market cap range, in millions of USD, of the coins the strategy buys or
// the signal announces. Each bound explains why it cannot be saved.
export function StrategyMarketCapFields({
	disabled,
	form,
}: StrategyMarketCapFieldsProps) {
	const errors = marketCapErrors(
		useWatchedValue(form, "minMarketCap", marketCapUsd),
		useWatchedValue(form, "maxMarketCap", marketCapUsd),
	);
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
						...form.getInputProps("minMarketCap"),
						error: errors.minimum,
						id: "strategy-min-market-cap",
						label: "Minimum",
					},
					{
						...field,
						...form.getInputProps("maxMarketCap"),
						error: errors.maximum,
						id: "strategy-max-market-cap",
						label: "Maximum",
					},
				]}
				title="Market cap"
			/>
			<Text c="dimmed" size="xs">
				Buys or signals only coins whose current market cap lies in this range;
				coins without a known market cap then buy or signal nothing. Backtests
				ignore the range. Leave empty for no limit.
			</Text>
		</Stack>
	);
}
