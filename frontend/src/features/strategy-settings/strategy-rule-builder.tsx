import { useMemo } from "react";
import { type Field, QueryBuilder, type RuleProps } from "react-querybuilder";
import type { StrategyVariable } from "@/api/generated/models";
import { StrategyGroup } from "@/features/strategy-settings/strategy-group";
import { StrategyGroupHeader } from "@/features/strategy-settings/strategy-group-header";
import { StrategyRule } from "@/features/strategy-settings/strategy-rule";
import type { StrategyQuery } from "@/features/strategy-settings/types";

interface StrategyRuleBuilderProps {
	disabled: boolean;
	onChange(query: StrategyQuery): void;
	query: StrategyQuery;
	variables: readonly StrategyVariable[];
}

// Builds the strategy as nested AND/OR groups of comparisons. The builder
// only manages the query; every control is a Mantine component.
export function StrategyRuleBuilder({
	disabled,
	onChange,
	query,
	variables,
}: StrategyRuleBuilderProps) {
	const fields = useMemo<Field[]>(
		() =>
			variables.map(({ label, name }) => ({
				label,
				name,
				valueSources: ["value", "field"],
			})),
		[variables],
	);
	const controlElements = useMemo(
		() => ({
			rule: (props: RuleProps) => (
				<StrategyRule {...props} variables={variables} />
			),
			ruleGroup: StrategyGroup,
			ruleGroupHeaderElements: StrategyGroupHeader,
		}),
		[variables],
	);
	return (
		<QueryBuilder
			controlElements={controlElements}
			disabled={disabled}
			fields={fields}
			getDefaultOperator={() => ">"}
			getDefaultValue={() => 0}
			onQueryChange={onChange}
			query={query}
			resetOnFieldChange={false}
			showNotToggle
		/>
	);
}
