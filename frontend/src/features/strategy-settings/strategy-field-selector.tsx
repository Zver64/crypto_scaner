import { type FieldSelectorProps, remove } from "react-querybuilder";

import { ExpressionEditor } from "@/features/strategy-settings/expression-editor";
import type { StrategyVariables } from "@/features/strategy-settings/types";
import {
	dispatchRuleUpdate,
	leftOperand,
	leftOperandUpdate,
	operandVariables,
} from "@/features/strategy-settings/utils";

// The compared side of a condition: an indicator, a function, or an indicator
// of another coin, never a number. Its first row ends with the condition's
// remove button.
export function StrategyFieldSelector({
	context,
	disabled,
	level,
	path,
	rule,
	schema,
}: FieldSelectorProps) {
	const variables: StrategyVariables = context;
	const RemoveRuleAction = schema.controls.removeRuleAction;
	return (
		<ExpressionEditor
			depth={level}
			action={
				<RemoveRuleAction
					disabled={disabled}
					handleOnClick={() =>
						schema.dispatchQuery(remove(schema.getQuery(), path))
					}
					level={level}
					path={path}
					ruleOrGroup={rule}
					schema={schema}
				/>
			}
			disabled={disabled === true}
			label="Compared value"
			node={leftOperand(rule)}
			onChange={(node) =>
				dispatchRuleUpdate(schema, path, leftOperandUpdate(node))
			}
			variables={operandVariables(variables, rule.operator)}
			withoutNumber
		/>
	);
}
