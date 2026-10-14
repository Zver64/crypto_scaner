import { useAppForm } from "@/components/form";
import type {
	StrategyFormData,
	StrategyFormValues,
} from "@/features/strategy-settings/types";
import {
	marketCapErrors,
	marketCapUsd,
	strategyFormValues,
} from "@/features/strategy-settings/utils";

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
		validators: {
			// The bounds are checked together, as the maximum must not be below
			// the minimum.
			onChange: ({ value }) => {
				const errors = marketCapErrors(
					marketCapUsd(value.minMarketCap),
					marketCapUsd(value.maxMarketCap),
				);
				return {
					fields: {
						maxMarketCap: errors.maximum,
						minMarketCap: errors.minimum,
					},
				};
			},
		},
	});
}
