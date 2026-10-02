import {
	ActionIcon,
	Group,
	NumberInput,
	Paper,
	Select,
	Stack,
} from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import { IconX } from "@tabler/icons-react";
import type { RuleProps } from "react-querybuilder";
import type { StrategyVariable } from "@/api/generated/models";
import { ExpressionEditor } from "@/features/strategy-settings/expression-editor";
import { firstField } from "@/features/strategy-settings/expressions";
import type { StrategyOperator } from "@/features/strategy-settings/types";
import {
	isCrossOperator,
	isRangeOperator,
	valueForOperator,
} from "@/features/strategy-settings/utils";

const operatorOptions: { label: string; value: StrategyOperator }[] = [
	{ label: "> above", value: ">" },
	{ label: "≥ at least", value: ">=" },
	{ label: "< below", value: "<" },
	{ label: "≤ at most", value: "<=" },
	{ label: "between", value: "between" },
	{ label: "not between", value: "notBetween" },
	{ label: "crosses above", value: "crossesAbove" },
	{ label: "crosses below", value: "crossesBelow" },
];

interface StrategyRuleProps extends RuleProps {
	variables: readonly StrategyVariable[];
}

// One comparison, stacked vertically for phones: the compared operand, the
// operator, then a range or the operand it is compared with. Each operand is
// an indicator, a number (right side only), a function, or an indicator of
// another coin.
export function StrategyRule({
	actions,
	disabled,
	path,
	rule,
	variables,
}: StrategyRuleProps) {
	const change = (
		prop: "field" | "lhs" | "operator" | "value" | "valueSource",
		value: unknown,
	) => actions.onPropChange(prop, value, path);
	// The field follows the expression, so the rule keeps naming a variable.
	const changeLhs = (lhs: ExpressionNode | undefined) => {
		change("lhs", lhs);
		const field = lhs ? firstField(lhs) : undefined;
		if (field) change("field", field);
	};
	const range = isRangeOperator(rule.operator);
	// Plain indicators and numbers stay plain rule fields and values; other
	// operands are expressions.
	const lhs: ExpressionNode = (rule.lhs as ExpressionNode | undefined) ?? {
		kind: "field",
		field: rule.field || (variables[0]?.name ?? ""),
	};
	const changeLeft = (node: ExpressionNode) => {
		if (node.kind === "field") {
			change("lhs", undefined);
			change("field", node.field);
		} else {
			changeLhs(node);
		}
	};
	const right: ExpressionNode =
		rule.valueSource === "expression"
			? (rule.value as ExpressionNode)
			: rule.valueSource === "field"
				? { kind: "field", field: String(rule.value ?? "") }
				: {
						kind: "value",
						value: typeof rule.value === "number" ? rule.value : Number.NaN,
					};
	const changeRight = (node: ExpressionNode) => {
		const source =
			node.kind === "field"
				? "field"
				: node.kind === "value"
					? "value"
					: "expression";
		if (source !== rule.valueSource) change("valueSource", source);
		change(
			"value",
			node.kind === "field"
				? node.field
				: node.kind === "value"
					? node.value
					: node,
		);
	};
	return (
		<Paper mt="xs" p="xs" radius="sm" withBorder>
			<Stack gap="xs">
				{/* The remove button shares the row of the kind selector, so the
				    editor below takes the full width. */}
				<ExpressionEditor
					action={
						<ActionIcon
							aria-label="Remove condition"
							color="red"
							disabled={disabled}
							onClick={() => actions.onRuleRemove(path)}
							variant="subtle"
						>
							<IconX size={16} />
						</ActionIcon>
					}
					disabled={disabled === true}
					label="Compared value"
					node={lhs}
					onChange={changeLeft}
					variables={variables}
					withoutNumber
				/>
				<Select
					aria-label="Comparison"
					allowDeselect={false}
					data={operatorOptions}
					disabled={disabled}
					onChange={(value) => {
						if (!value) return;
						change("operator", value);
						const source = isRangeOperator(value) ? "value" : rule.valueSource;
						if (source !== rule.valueSource) change("valueSource", source);
						change("value", valueForOperator(value, rule.value, source));
					}}
					size="sm"
					value={rule.operator}
				/>
				{range ? (
					<Group gap="xs" grow wrap="nowrap">
						{[0, 1].map((index) => (
							<NumberInput
								aria-label={index === 0 ? "From" : "To"}
								disabled={disabled}
								key={index}
								onChange={(value) => {
									const bounds: unknown[] = Array.isArray(rule.value)
										? [...rule.value]
										: [0, 0];
									bounds[index] = value;
									change("value", bounds);
								}}
								placeholder={index === 0 ? "From" : "To"}
								size="sm"
								value={Array.isArray(rule.value) ? rule.value[index] : ""}
							/>
						))}
					</Group>
				) : (
					<ExpressionEditor
						disabled={disabled === true}
						label={
							isCrossOperator(rule.operator)
								? "Crossed value"
								: "Value compared with"
						}
						defaultField={
							variables.find(({ name }) => name !== rule.field)?.name
						}
						node={right}
						onChange={changeRight}
						variables={variables}
					/>
				)}
			</Stack>
		</Paper>
	);
}
