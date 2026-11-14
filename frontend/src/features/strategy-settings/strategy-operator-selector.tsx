import { NativeSelect } from "@mantine/core";
import {
	type OperatorSelectorProps,
	toFlatOptionArray,
} from "react-querybuilder";

import {
	dispatchRuleUpdate,
	operatorUpdate,
} from "@/features/strategy-settings/utils";

// The comparison of a condition. The value follows the chosen operator in the
// same update.
export function StrategyOperatorSelector({
	disabled,
	options,
	path,
	rule,
	schema,
	value,
}: OperatorSelectorProps) {
	return (
		<NativeSelect
			aria-label="Comparison"
			data={toFlatOptionArray(options).map(({ label, value }) => ({
				label,
				value,
			}))}
			disabled={disabled}
			onChange={(event) =>
				dispatchRuleUpdate(
					schema,
					path,
					operatorUpdate(rule, event.currentTarget.value),
				)
			}
			size="sm"
			value={value}
		/>
	);
}
