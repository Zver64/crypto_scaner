import { QueryBuilder, type ValueSources } from "react-querybuilder";

import { strategyOperators } from "@/features/strategy-settings/constants";
import { StrategyFieldSelector } from "@/features/strategy-settings/strategy-field-selector";
import { StrategyGroup } from "@/features/strategy-settings/strategy-group";
import { StrategyGroupHeader } from "@/features/strategy-settings/strategy-group-header";
import { StrategyOperatorSelector } from "@/features/strategy-settings/strategy-operator-selector";
import { StrategyRemoveRuleAction } from "@/features/strategy-settings/strategy-remove-rule-action";
import { StrategyRule } from "@/features/strategy-settings/strategy-rule";
import { StrategyValueEditor } from "@/features/strategy-settings/strategy-value-editor";
import type {
	StrategyQuery,
	StrategyRuleVariables,
} from "@/features/strategy-settings/types";
import { isRangeOperator } from "@/features/strategy-settings/utils";

interface StrategyRuleBuilderProps {
	// The query the builder starts with; the builder keeps its own query
	// afterwards, so replacing it means remounting the builder.
	defaultQuery: StrategyQuery;
	disabled: boolean;
	onQueryChange(query: StrategyQuery): void;
	// Stable across renders, so a render of the builder keeps its schema
	// and leaves the conditions alone.
	variables: StrategyRuleVariables;
}

// Module-level, so the builder keeps one component per control and its own
// memoization leaves unchanged groups and conditions alone.
const controlElements = {
	fieldSelector: StrategyFieldSelector,
	operatorSelector: StrategyOperatorSelector,
	removeRuleAction: StrategyRemoveRuleAction,
	rule: StrategyRule,
	valueEditor: StrategyValueEditor,
	// The operand kind menu of the value editor chooses the value source.
	valueSourceSelector: null,
	ruleGroup: StrategyGroup,
	ruleGroupHeaderElements: StrategyGroupHeader,
};

const getDefaultOperator = () => ">";

const getDefaultValue = () => 0;

// A range compares two numbers; other operands are a number, an indicator, or
// an expression.
const getValueSources = (_field: string, operator: string): ValueSources =>
	isRangeOperator(operator) ? ["value"] : ["value", "field", "expression"];

// Groups nest at most two levels below the top level.
const maxLevels = 2;

// Builds the strategy as nested AND/OR groups of comparisons. The builder is
// uncontrolled: it renders on its own edits and reports them, and every
// control is a Mantine component.
export function StrategyRuleBuilder({
	defaultQuery,
	disabled,
	onQueryChange,
	variables,
}: StrategyRuleBuilderProps) {
	return (
		<QueryBuilder
			// Every control reads the variables its operands offer.
			context={variables}
			controlElements={controlElements}
			defaultQuery={defaultQuery}
			disabled={disabled}
			// The opened query is already in the form.
			enableMountQueryChange={false}
			fields={variables.fields}
			getDefaultField={variables.defaultField}
			getDefaultOperator={getDefaultOperator}
			getDefaultValue={getDefaultValue}
			getValueSources={getValueSources}
			maxLevels={maxLevels}
			onQueryChange={onQueryChange}
			operators={strategyOperators}
			resetOnFieldChange={false}
			showNotToggle
		/>
	);
}
