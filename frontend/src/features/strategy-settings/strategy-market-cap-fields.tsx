import { Fieldset, Stack, Text } from "@mantine/core";

import type { StrategyFormApi } from "@/features/strategy-settings/types";

interface StrategyMarketCapFieldsProps {
	disabled: boolean;
	form: StrategyFormApi;
}

// The market cap range, in millions of USD, of the coins the strategy buys or
// the signal announces. The form checks both bounds together.
export function StrategyMarketCapFields({
	disabled,
	form,
}: StrategyMarketCapFieldsProps) {
	const input = {
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
					<form.AppField name="minMarketCap">
						{(field) => <field.NumberField {...input} label="Minimum" />}
					</form.AppField>
					<form.AppField name="maxMarketCap">
						{(field) => <field.NumberField {...input} label="Maximum" />}
					</form.AppField>
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
