import { Fieldset, NumberInput, Stack, Text } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";

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
			<Fieldset legend="Market cap">
				<Stack gap="xs">
					<NumberInput
						{...field}
						{...form.getInputProps("minMarketCap")}
						error={errors.minimum}
						label="Minimum"
					/>
					<NumberInput
						{...field}
						{...form.getInputProps("maxMarketCap")}
						error={errors.maximum}
						label="Maximum"
					/>
				</Stack>
			</Fieldset>
			<Text c="dimmed" size="xs">
				Buys or signals only coins whose current market cap lies in this range;
				coins without a known market cap then buy or signal nothing. Backtests
				ignore the range. Leave empty for no limit.
			</Text>
		</Stack>
	);
}
