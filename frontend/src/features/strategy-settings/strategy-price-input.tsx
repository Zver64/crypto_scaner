import { Button, Group, Stack, Text, TextInput } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import type { StrategyVariable } from "@/api/generated/models";
import { ExpressionEditor } from "@/features/strategy-settings/expression-editor";
import type { PriceSetup } from "@/features/strategy-settings/strategy-price-input/types";
import type {
	StrategyFormValues,
	StrategyPriceField,
} from "@/features/strategy-settings/types";
import { useWatchedValue } from "@/features/strategy-settings/use-watched-value";

interface StrategyPriceInputProps {
	disabled: boolean;
	field: StrategyPriceField;
	form: UseFormReturnType<StrategyFormValues>;
	label: string;
	variables: readonly StrategyVariable[];
}

// The same numeric operand builder as conditions, evaluated once at entry.
// It follows the form field, which a direction change clears.
export function StrategyPriceInput({
	disabled,
	field,
	form,
	label,
	variables,
}: StrategyPriceInputProps) {
	const value = useWatchedValue(form, field, (setup) => setup);
	const onChange = (setup: PriceSetup) =>
		form.setFieldValue(field, setup, { forceUpdate: false });
	const enabled = value.node !== undefined || value.custom !== undefined;
	const start = () => onChange({ node: { kind: "field", field: "" } });
	return (
		<Stack gap={6}>
			<Group justify="space-between">
				<Text fw={500} size="sm">
					{label}
				</Text>
				<Button
					disabled={disabled}
					onClick={enabled ? () => onChange({}) : start}
					size="compact-sm"
					variant="subtle"
				>
					{enabled ? "Remove" : "Add"}
				</Button>
			</Group>
			{value.node ? (
				<ExpressionEditor
					disabled={disabled}
					label={label}
					node={value.node}
					onChange={(node) => onChange({ node })}
					variables={variables}
					withoutCoin
				/>
			) : value.custom !== undefined ? (
				<Stack gap={6}>
					<TextInput
						disabled={disabled}
						label="Stored formula"
						maxLength={2000}
						onChange={(event) =>
							onChange({ custom: event.currentTarget.value })
						}
						styles={(theme) => ({
							input: { fontFamily: theme.fontFamilyMonospace },
						})}
						value={value.custom}
					/>
					<Button
						disabled={disabled}
						onClick={start}
						size="compact-sm"
						variant="subtle"
					>
						Replace with builder
					</Button>
				</Stack>
			) : null}
		</Stack>
	);
}
