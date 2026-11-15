import {
	ActionIcon,
	Box,
	Group,
	Menu,
	NativeSelect,
	NumberInput,
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

import { CoinSelect } from "@/features/strategy-settings/coin-select";
import {
	coinCall,
	coinOf,
	firstField,
	functionCall,
	indicatorCall,
	indicatorCallSource,
	replaceFunction,
	strategyFunctions,
	withCoin,
} from "@/features/strategy-settings/expressions";
import { FunctionEditor } from "@/features/strategy-settings/function-editor";
import { IndicatorParameters } from "@/features/strategy-settings/indicator-parameters";
import { NestingLine } from "@/features/strategy-settings/nesting-line";
import { StrategyVariableInput } from "@/features/strategy-settings/strategy-variable-input";
import type { StrategyVariables } from "@/features/strategy-settings/types";
import {
	argumentIssuePaths,
	defaultVariableCall,
	fieldIssuePaths,
	numberInputValue,
	ownIssuePaths,
	sourceVariables,
	withoutPositionVariables,
} from "@/features/strategy-settings/utils";

export interface ExpressionEditorProps {
	disabled: boolean;
	// Names the operand for assistive technology; the operand row does not show
	// it.
	label: string;
	node: ExpressionNode;
	onChange(node: ExpressionNode): void;
	variables: StrategyVariables;
	// Hides the number kind where a number alone makes no sense, such as the
	// compared side of a condition.
	withoutNumber?: boolean;
	// The call a new indicator or coin operand starts with when the operand
	// reads none, such as one other than the compared indicator.
	defaultField?: string;
	// Hides the coin kind inside an operand of another coin, since of
	// cannot contain of.
	withoutCoin?: boolean;
	// How many groups and functions enclose the operand, which picks the
	// line color of its arguments.
	depth?: number;
	// A control at the end of the operand row, such as a remove button.
	action?: ReactNode;
	// The paths, relative to node, of the validation issues of the condition
	// it is part of; each input an issue points at gets an error border.
	issuePaths?: readonly PropertyKey[][];
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
	issuePaths = [],
}: ExpressionEditorProps) {
	const kind = operandKind(node);
	const ownPaths = ownIssuePaths(issuePaths, kind === "coin");
	// An indicator call's own issues are those of the indicator; the others
	// point at its parameters.
	const fieldPaths = fieldIssuePaths(ownPaths);
	const incomplete =
		kind === "field"
			? fieldPaths.some((path) => path.length === 0)
			: ownPaths.length > 0;
	const call =
		node.kind === "field"
			? indicatorCall(node.field, variables.variables)
			: undefined;
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
		field:
			firstField(node) ?? defaultField ?? defaultVariableCall(variables) ?? "",
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
							invalid={incomplete}
							onChange={(symbol) => onChange(withCoin(node, symbol))}
						/>
					) : node.kind === "func" ? (
						<NativeSelect
							aria-label="Function"
							data={functionOptions}
							disabled={disabled}
							error={incomplete}
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
							error={incomplete}
							onChange={(value) => onChange({ kind: "value", value })}
							size="sm"
							value={numberInputValue(node.value)}
						/>
					) : (
						<StrategyVariableInput
							disabled={disabled}
							field={node.kind === "field" ? node.field : ""}
							invalid={incomplete}
							label={label}
							onChange={(field) => onChange({ kind: "field", field })}
							variables={variables}
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
							issuePaths={argumentIssuePaths(issuePaths, 1)}
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
							issuePaths={issuePaths}
							node={node}
							onChange={onChange}
							variables={variables}
							withoutCoin={withoutCoin}
						/>
					)}
				</NestingLine>
			) : null}
			{call && call.args.length > 0 ? (
				<NestingLine depth={depth}>
					<IndicatorParameters
						call={call}
						depth={depth}
						disabled={disabled}
						issuePaths={fieldPaths}
						onChange={(next) =>
							onChange({ kind: "field", field: indicatorCallSource(next) })
						}
						sources={sourceVariables(variables.variables)}
					/>
				</NestingLine>
			) : null}
		</Stack>
	);
}
