import {
	Group,
	NumberInput,
	Paper,
	SegmentedControl,
	Select,
	Stack,
	useMantineTheme,
} from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import type { ReactNode } from "react";
import type { StrategyVariable } from "@/api/generated/models";
import { CoinSelect } from "@/features/strategy-settings/coin-select";
import {
	coinCall,
	coinOf,
	firstField,
	functionCall,
	withCoin,
} from "@/features/strategy-settings/expressions";
import { FunctionEditor } from "@/features/strategy-settings/function-editor";
import { nestingBorder } from "@/features/strategy-settings/nesting-colors";
import { variableSelectData } from "@/features/strategy-settings/utils";

export interface ExpressionEditorProps {
	disabled: boolean;
	// Names the operand for assistive technology; it is not shown, so the
	// kind selector keeps the full width on phones.
	label: string;
	node: ExpressionNode;
	onChange(node: ExpressionNode): void;
	variables: readonly StrategyVariable[];
	// Hides the number kind where a number alone makes no sense, such as the
	// compared side of a condition.
	withoutNumber?: boolean;
	// The indicator a new indicator or coin operand starts with when the
	// operand reads none, such as one other than the compared indicator.
	defaultField?: string;
	// Hides the coin kind inside an operand of another coin, since of
	// cannot contain of.
	withoutCoin?: boolean;
	// How many groups and functions enclose the operand, which picks the
	// border color of its function.
	depth?: number;
	// A control beside the kind selector, such as a remove button, which
	// leaves the full width to the operand below.
	action?: ReactNode;
}

type OperandKind = "field" | "value" | "func" | "coin";

function operandKind(node: ExpressionNode): OperandKind {
	if (node.kind === "value") return "value";
	if (node.kind !== "func") return "field";
	return coinOf(node) === undefined ? "func" : "coin";
}

// Edits one operand: an indicator or candle field, a number, a function
// whose arguments are operands themselves, or an operand of another coin.
export function ExpressionEditor({
	disabled,
	label,
	node,
	onChange,
	variables,
	withoutNumber = false,
	withoutCoin = false,
	defaultField,
	action,
	depth = 0,
}: ExpressionEditorProps) {
	const kind = operandKind(node);
	const border = nestingBorder(useMantineTheme(), depth);
	// A kind already chosen stays selectable, so the control shows it.
	const kinds: { label: string; value: OperandKind }[] = [
		{ label: "Indicator", value: "field" },
		...(!withoutNumber || kind === "value"
			? [{ label: "Number", value: "value" as const }]
			: []),
		{ label: "Function", value: "func" },
		...(!withoutCoin || kind === "coin"
			? [{ label: "Coin", value: "coin" as const }]
			: []),
	];
	// A new kind keeps the first indicator the operand reads.
	const field: ExpressionNode = {
		kind: "field",
		field: firstField(node) ?? defaultField ?? variables[0]?.name ?? "",
	};
	const editor = (
		<Stack gap={4}>
			<Group gap="xs" wrap="nowrap">
				<SegmentedControl
					aria-label={`${label} kind`}
					data={kinds}
					disabled={disabled}
					flex={1}
					onChange={(next) => {
						if (next === "func") onChange(functionCall("multiply", node));
						else if (next === "coin") onChange(coinCall(field));
						else if (next === "value") onChange({ kind: "value", value: 0 });
						else onChange(field);
					}}
					size="xs"
					value={kind}
				/>
				{action}
			</Group>
			{node.kind === "func" && kind === "coin" ? (
				<Stack gap={4}>
					<CoinSelect
						coin={coinOf(node) ?? ""}
						disabled={disabled}
						onChange={(symbol) => onChange(withCoin(node, symbol))}
					/>
					<ExpressionEditor
						depth={depth}
						disabled={disabled}
						label={`${label} of the coin`}
						node={node.args[1] ?? field}
						onChange={(operand) =>
							onChange({ ...node, args: [node.args[0] ?? field, operand] })
						}
						variables={variables}
						withoutCoin
						withoutNumber
					/>
				</Stack>
			) : node.kind === "func" ? (
				<FunctionEditor
					depth={depth}
					disabled={disabled}
					node={node}
					onChange={onChange}
					variables={variables}
					withoutCoin={withoutCoin}
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
	// A function is framed as a whole, with its kind, name, and arguments.
	return kind === "func" ? (
		<Paper p="xs" radius="sm" style={{ borderColor: border }} withBorder>
			{editor}
		</Paper>
	) : (
		editor
	);
}
