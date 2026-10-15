import { useAppForm } from "@/components/form";
import type {
	StrategyFormData,
	StrategyFormValues,
} from "@/features/strategy-settings/types";
import {
	marketCapErrors,
	marketCapUsd,
	strategyCoinsError,
	strategyFormValues,
} from "@/features/strategy-settings/utils";

// Checks the fields that depend on others: the coins on the coin source,
// and the market cap bounds together, as the maximum must not be below the
// minimum.
function validateLinkedFields({ value }: { value: StrategyFormData }) {
	const errors = marketCapErrors(
		marketCapUsd(value.minMarketCap),
		marketCapUsd(value.maxMarketCap),
	);
	return {
		fields: {
			coins: strategyCoinsError(value.coins, value.coinSource),
			maxMarketCap: errors.maximum,
			minMarketCap: errors.minimum,
		},
	};
}

// The form of the open draft, opened with its values; without a draft it
// holds nothing to show. It submits only values strategyInput accepts.
export function useStrategyForm(
	values: StrategyFormValues | undefined,
	onSubmit: (values: StrategyFormValues) => void,
) {
	const defaultValues: StrategyFormData | undefined = values;
	return useAppForm({
		defaultValues,
		onSubmit: ({ value }) => onSubmit(strategyFormValues(value)),
		validators: { onChange: validateLinkedFields },
	});
}
