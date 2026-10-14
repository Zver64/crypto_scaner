import { Button } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import type {
	StrategyFormApi,
	StrategyRuleField,
} from "@/features/strategy-settings/types";
import { strategyFormValues } from "@/features/strategy-settings/utils";

interface StrategyConditionsReplaceButtonProps {
	disabled: boolean;
	field: StrategyRuleField;
	form: StrategyFormApi;
	onClear(): void;
	onImport(): void;
}

// An empty builder imports an expression; conditions are cleared first, so
// an import never replaces them.
export function StrategyConditionsReplaceButton({
	disabled,
	field,
	form,
	onClear,
	onImport,
}: StrategyConditionsReplaceButtonProps) {
	const empty = useSelector(form.store, ({ values }) => {
		const query = strategyFormValues(values)[field];
		return query === undefined || query.rules.length === 0;
	});
	return empty ? (
		<Button
			disabled={disabled}
			onClick={onImport}
			size="compact-sm"
			variant="subtle"
		>
			Import
		</Button>
	) : (
		<Button
			color="red"
			disabled={disabled}
			onClick={onClear}
			size="compact-sm"
			variant="subtle"
		>
			Clear
		</Button>
	);
}
