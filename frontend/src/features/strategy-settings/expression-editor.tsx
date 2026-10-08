import {
	ActionIcon,
	Box,
	Group,
	Menu,
	NativeSelect,
	NumberInput,
	Select,
	Stack,
} from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import {
	type Icon,
	IconChartLine,
	IconCoin,
	IconHash,
	IconMathFunction,
} from "@tabler/icons-react";
import type { ReactNode } from "react";
import type { StrategyVariable } from "@/api/generated/models";
import { CoinSelect } from "@/features/strategy-settings/coin-select";
import {
	coinCall,
	coinOf,
	firstField,
	functionCall,
	replaceFunction,
	strategyFunctions,
	withCoin,
} from "@/features/strategy-settings/expressions";
import { FunctionEditor } from "@/features/strategy-settings/function-editor";
import { NestingLine } from "@/features/strategy-settings/nesting-line";
import {
	variableSelectData,
	withoutPositionVariables,
} from "@/features/strategy-settings/utils";

export interface ExpressionEditorProps {
	disabled: boolean;
	// Names the operand for assistive technology; the operand row does not show
	// it.
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
	// line color of its arguments.
	depth?: number;
	// A control at the end of the operand row, such as a remove button.
	action?: ReactNode;
}

type OperandKind = "field" | "value" | "func" | "coin";

// The kinds in menu order.
const operandKinds = {
	field: { icon: IconChartLine, label: "Indicator" },
	value: { icon: IconHash, label: "Number" },
	func: { icon: IconMathFunction, label: "Function" },
	coin: { icon: IconCoin, label: "Another coin" },
} as const satisfies Record<OperandKind, { icon: Icon; label: string }>;

const operandKindOrder = Object.keys(operandKinds) as OperandKind[];

const functionOptions = Object.entries(strategyFunctions).map(
	([value, { label }]) => ({ label, value }),
);

function operandKind(node: ExpressionNode): OperandKind {
	if (node.kind === "value") return "value";
	if (node.kind !== "func") return "field";
	return coinOf(node) === undefined ? "func" : "coin";
}

// Edits one operand in a single row: a kind button, then an indicator, a
// number, a function, or the coin of an operand of another coin. Function
// arguments and the operand of another coin follow below, indented behind a
// line in the color of their depth.
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
	// A kind already chosen stays offered, so the menu shows it.
	const kinds = operandKindOrder.filter(
		(value) =>
			value === kind ||
			!(
				(value === "value" && withoutNumber) ||
				(value === "coin" && withoutCoin)
			),
	);
	const KindIcon = operandKinds[kind].icon;
	// A new kind keeps the first indicator the operand reads.
	const field: ExpressionNode = {
		kind: "field",
		field: firstField(node) ?? defaultField ?? variables[0]?.name ?? "",
	};
	const choose = (next: OperandKind) => {
		if (next === kind) return;
		if (next === "func") onChange(functionCall("multiply", node));
		else if (next === "coin") onChange(coinCall(field));
		else if (next === "value") onChange({ kind: "value", value: 0 });
		else onChange(field);
	};
	return (
		<Stack gap={6}>
			<Group gap={6} wrap="nowrap">
				<Menu position="bottom-start">
					<Menu.Target>
						<ActionIcon
							aria-label={`${label} kind`}
							disabled={disabled}
							size="input-sm"
							variant="default"
						>
							<KindIcon size={18} />
						</ActionIcon>
					</Menu.Target>
					<Menu.Dropdown>
						{kinds.map((value) => {
							const { icon: KindItemIcon, label: kindLabel } =
								operandKinds[value];
							return (
								<Menu.Item
									key={value}
									leftSection={<KindItemIcon size={16} />}
									onClick={() => choose(value)}
								>
									{kindLabel}
								</Menu.Item>
							);
						})}
					</Menu.Dropdown>
				</Menu>
				<Box flex={1} miw={0}>
					{node.kind === "func" && kind === "coin" ? (
						<CoinSelect
							coin={coinOf(node) ?? ""}
							disabled={disabled}
							onChange={(symbol) => onChange(withCoin(node, symbol))}
						/>
					) : node.kind === "func" ? (
						<NativeSelect
							aria-label="Function"
							data={functionOptions}
							disabled={disabled}
							onChange={(event) =>
								onChange(replaceFunction(node, event.currentTarget.value))
							}
							size="sm"
							value={node.fn}
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
							clearable
							data={variableSelectData(variables)}
							disabled={disabled}
							// Clearing leaves the indicator to be chosen again.
							onChange={(value) =>
								onChange({ kind: "field", field: value ?? "" })
							}
							placeholder="Indicator"
							searchable
							size="sm"
							value={
								node.kind === "field" && node.field !== "" ? node.field : null
							}
						/>
					)}
				</Box>
				{action}
			</Group>
			{node.kind === "func" ? (
				<NestingLine depth={depth}>
					{kind === "coin" ? (
						<ExpressionEditor
							depth={depth + 1}
							disabled={disabled}
							label={`${label} of the coin`}
							node={node.args[1] ?? field}
							onChange={(operand) =>
								onChange({ ...node, args: [node.args[0] ?? field, operand] })
							}
							variables={withoutPositionVariables(variables)}
							withoutCoin
							withoutNumber
						/>
					) : (
						<FunctionEditor
							depth={depth}
							disabled={disabled}
							node={node}
							onChange={onChange}
							variables={variables}
							withoutCoin={withoutCoin}
						/>
					)}
				</NestingLine>
			) : null}
		</Stack>
	);
}
