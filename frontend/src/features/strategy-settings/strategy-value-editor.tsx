import { Group, NumberInput } from "@mantine/core";
import { isRuleOrGroupValid, type ValueEditorProps } from "react-querybuilder";

import { ExpressionEditor } from "@/features/strategy-settings/expression-editor";
import { operandComplete } from "@/features/strategy-settings/expressions";
import type { StrategyVariables } from "@/features/strategy-settings/types";
import {
	defaultVariableCall,
	dispatchRuleUpdate,
	isCrossOperator,
	isRangeOperator,
	operandVariables,
	rightOperand,
	rightOperandUpdate,
} from "@/features/strategy-settings/utils";

// The side a condition compares with: a range of two numbers, or a number, an
// indicator, a function, or an indicator of another coin.
export function StrategyValueEditor({
	context,
	disabled,
	handleOnChange,
	level,
	operator,
	path,
	rule,
	schema,
	validation,
	value,
}: ValueEditorProps) {
	const invalid = !isRuleOrGroupValid(rule, validation);
	if (isRangeOperator(operator)) {
		return (
			<Group gap="xs" grow wrap="nowrap">
				{[0, 1].map((index) => (
					<NumberInput
						aria-label={index === 0 ? "From" : "To"}
						disabled={disabled}
						error={
							invalid &&
							!operandComplete({
								kind: "value",
								value: Array.isArray(value) ? value[index] : undefined,
							})
						}
						key={index}
						onChange={(bound) => {
							const bounds: unknown[] = Array.isArray(value)
								? [...value]
								: [0, 0];
							bounds[index] = bound;
							handleOnChange(bounds);
						}}
						placeholder={index === 0 ? "From" : "To"}
						size="sm"
						value={Array.isArray(value) ? value[index] : ""}
					/>
				))}
			</Group>
		);
	}
	const strategyVariables: StrategyVariables = context;
	const variables = operandVariables(strategyVariables, operator);
	return (
		<ExpressionEditor
			depth={level}
			disabled={disabled === true}
			label={
				isCrossOperator(operator) ? "Crossed value" : "Value compared with"
			}
			defaultField={defaultVariableCall(variables, rule.field)}
			markIncomplete={invalid}
			node={rightOperand(rule)}
			onChange={(node) =>
				dispatchRuleUpdate(schema, path, rightOperandUpdate(node))
			}
			variables={variables}
		/>
	);
}
