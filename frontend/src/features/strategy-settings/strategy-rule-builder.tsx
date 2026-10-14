import { QueryBuilder } from "react-querybuilder";

import { StrategyGroup } from "@/features/strategy-settings/strategy-group";
import { StrategyGroupHeader } from "@/features/strategy-settings/strategy-group-header";
import { StrategyRule } from "@/features/strategy-settings/strategy-rule";
import { StrategyVariablesContext } from "@/features/strategy-settings/strategy-variables-context";
import type {
	StrategyQuery,
	StrategyRuleVariables,
} from "@/features/strategy-settings/types";

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
	rule: StrategyRule,
	ruleGroup: StrategyGroup,
	ruleGroupHeaderElements: StrategyGroupHeader,
};

const getDefaultOperator = () => ">";

const getDefaultValue = () => 0;

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
		<StrategyVariablesContext value={variables.variables}>
			<QueryBuilder
				controlElements={controlElements}
				defaultQuery={defaultQuery}
				disabled={disabled}
				// The opened query is already in the form.
				enableMountQueryChange={false}
				fields={variables.fields}
				getDefaultOperator={getDefaultOperator}
				getDefaultValue={getDefaultValue}
				onQueryChange={onQueryChange}
				resetOnFieldChange={false}
				showNotToggle
			/>
		</StrategyVariablesContext>
	);
}
