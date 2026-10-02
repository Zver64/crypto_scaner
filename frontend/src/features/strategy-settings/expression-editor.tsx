import { NumberInput, SegmentedControl, Select, Stack } from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import type { StrategyVariable } from "@/api/generated/models";
import { functionCall } from "@/features/strategy-settings/expressions";
import { FunctionEditor } from "@/features/strategy-settings/function-editor";
import { variableSelectData } from "@/features/strategy-settings/utils";

export interface ExpressionEditorProps {
	disabled: boolean;
	// Names the operand for assistive technology; it is not shown, so the
	// kind selector keeps the full width on phones.
	label: string;
	node: ExpressionNode;
	onChange(node: ExpressionNode): void;
	variables: readonly StrategyVariable[];
}

// Edits one operand: an indicator or candle field, a number, or a function
// whose arguments are operands themselves.
export function ExpressionEditor({
	disabled,
	label,
	node,
	onChange,
	variables,
}: ExpressionEditorProps) {
	const kind =
		node.kind === "func" || node.kind === "value" ? node.kind : "field";
	return (
		<Stack gap={4}>
			<SegmentedControl
				aria-label={`${label} kind`}
				data={[
					{ label: "Indicator", value: "field" },
					{ label: "Number", value: "value" },
					{ label: "Function", value: "func" },
				]}
				disabled={disabled}
				onChange={(next) => {
					if (next === "func") onChange(functionCall("multiply", node));
					else if (next === "value") onChange({ kind: "value", value: 0 });
					else onChange({ kind: "field", field: variables[0]?.name ?? "" });
				}}
				fullWidth
				size="xs"
				value={kind}
			/>
			{node.kind === "func" ? (
				<FunctionEditor
					disabled={disabled}
					node={node}
					onChange={onChange}
					variables={variables}
				/>
			) : node.kind === "value" ? (
				<NumberInput
					aria-label={label}
					disabled={disabled}
					onChange={(value) =>
						onChange({
							kind: "value",
							value: typeof value === "number" ? value : Number.NaN,
						})
					}
					size="sm"
					value={
						typeof node.value === "number" && Number.isFinite(node.value)
							? node.value
							: ""
					}
				/>
			) : (
				<Select
					aria-label={label}
					data={variableSelectData(variables)}
					disabled={disabled}
					onChange={(value) => {
						if (value) onChange({ kind: "field", field: value });
					}}
					searchable
					size="sm"
					value={node.kind === "field" && node.field !== "" ? node.field : null}
				/>
			)}
		</Stack>
	);
}
