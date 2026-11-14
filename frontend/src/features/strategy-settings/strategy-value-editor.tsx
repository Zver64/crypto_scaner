import { Group, NumberInput } from "@mantine/core";
import type { ValueEditorProps } from "react-querybuilder";

import { ExpressionEditor } from "@/features/strategy-settings/expression-editor";
import type { StrategyVariables } from "@/features/strategy-settings/types";
import {
	defaultVariableCall,
	dispatchRuleUpdate,
	isCrossOperator,
	isRangeOperator,
	issuePathsUnder,
	numberInputValue,
	operandVariables,
	rightOperand,
	rightOperandUpdate,
	validationIssuePaths,
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
	const issuePaths = issuePathsUnder(validationIssuePaths(validation), "value");
	if (isRangeOperator(operator)) {
		return (
			<Group gap="xs" grow wrap="nowrap">
				{[0, 1].map((index) => (
					<NumberInput
						aria-label={index === 0 ? "From" : "To"}
						disabled={disabled}
						// A bound is marked by its own issue, both by one about the range.
						error={issuePaths.some(
							([bound]) => bound === undefined || bound === index,
						)}
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
						value={numberInputValue(
							Array.isArray(value) ? value[index] : undefined,
						)}
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
			issuePaths={issuePaths}
			node={rightOperand(rule)}
			onChange={(node) =>
				dispatchRuleUpdate(schema, path, rightOperandUpdate(node))
			}
			variables={variables}
		/>
	);
}
