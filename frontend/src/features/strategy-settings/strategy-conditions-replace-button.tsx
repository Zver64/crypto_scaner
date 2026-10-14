import { Button } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import type {
	StrategyFormValues,
	StrategyRuleField,
} from "@/features/strategy-settings/types";
import { useWatchedValue } from "@/features/strategy-settings/use-watched-value";

interface StrategyConditionsReplaceButtonProps {
	disabled: boolean;
	field: StrategyRuleField;
	form: UseFormReturnType<StrategyFormValues>;
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
	const empty = useWatchedValue(
		form,
		field,
		(query) => query === undefined || query.rules.length === 0,
	);
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
