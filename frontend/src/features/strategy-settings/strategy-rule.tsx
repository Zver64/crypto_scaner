import {
	ActionIcon,
	Group,
	NumberInput,
	Paper,
	SegmentedControl,
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
	variableSelectData,
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

// One comparison, stacked vertically for phones: an indicator or an
// expression, the operator, then a number, a range, another indicator, or an
// expression.
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
	const variableOptions = variableSelectData(variables);
	const range = isRangeOperator(rule.operator);
	const byIndicator = rule.valueSource === "field";
	const byExpression = rule.valueSource === "expression";
	const fieldNode: ExpressionNode = {
		kind: "field",
		field: rule.field || (variables[0]?.name ?? ""),
	};
	return (
		<Paper mt="xs" p="xs" radius="sm" withBorder>
			<Stack gap="xs">
				{/* The remove button shares the first row, so the controls below
				    take the full width. */}
				<Group gap="xs" wrap="nowrap">
					<SegmentedControl
						data={[
							{ label: "Indicator", value: "field" },
							{ label: "Expression", value: "expression" },
						]}
						disabled={disabled}
						flex={1}
						onChange={(source) =>
							changeLhs(source === "expression" ? fieldNode : undefined)
						}
						size="xs"
						value={rule.lhs ? "expression" : "field"}
					/>
					<ActionIcon
						aria-label="Remove condition"
						color="red"
						disabled={disabled}
						onClick={() => actions.onRuleRemove(path)}
						variant="subtle"
					>
						<IconX size={16} />
					</ActionIcon>
				</Group>
				{rule.lhs ? (
					<ExpressionEditor
						disabled={disabled === true}
						label="Compared value"
						node={rule.lhs as ExpressionNode}
						onChange={changeLhs}
						variables={variables}
					/>
				) : (
					<Select
						aria-label="Indicator"
						data={variableOptions}
						disabled={disabled}
						onChange={(value) => {
							if (value) change("field", value);
						}}
						searchable
						size="sm"
						value={rule.field || null}
					/>
				)}
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
				{range ? null : (
					<SegmentedControl
						data={[
							{ label: "Number", value: "value" },
							{ label: "Indicator", value: "field" },
							{ label: "Expression", value: "expression" },
						]}
						disabled={disabled}
						onChange={(source) => {
							const other =
								variables.find(({ name }) => name !== rule.field)?.name ?? "";
							change("valueSource", source);
							change(
								"value",
								source === "field"
									? other
									: source === "expression"
										? ({
												kind: "field",
												field: other,
											} satisfies ExpressionNode)
										: 0,
							);
						}}
						size="xs"
						value={
							byExpression ? "expression" : byIndicator ? "field" : "value"
						}
					/>
				)}
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
				) : byExpression ? (
					<ExpressionEditor
						disabled={disabled === true}
						label={
							isCrossOperator(rule.operator)
								? "Crossed value"
								: "Value compared with"
						}
						node={rule.value as ExpressionNode}
						onChange={(node) => change("value", node)}
						variables={variables}
					/>
				) : byIndicator ? (
					<Select
						aria-label={
							isCrossOperator(rule.operator)
								? "Crossed indicator"
								: "Compared indicator"
						}
						data={variableOptions}
						disabled={disabled}
						onChange={(value) => {
							if (value) change("value", value);
						}}
						searchable
						size="sm"
						value={
							typeof rule.value === "string" && rule.value !== ""
								? rule.value
								: null
						}
					/>
				) : (
					<NumberInput
						aria-label="Number"
						disabled={disabled}
						onChange={(value) => change("value", value)}
						size="sm"
						value={typeof rule.value === "number" ? rule.value : ""}
					/>
				)}
			</Stack>
		</Paper>
	);
}
