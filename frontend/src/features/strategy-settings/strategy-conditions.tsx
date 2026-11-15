import { Group, Stack, Text } from "@mantine/core";
import { type ReactNode, useState } from "react";

import { StrategyConditionsReplaceButton } from "@/features/strategy-settings/strategy-conditions-replace-button";
import { StrategyExpressionPreview } from "@/features/strategy-settings/strategy-expression-preview";
import { StrategyImport } from "@/features/strategy-settings/strategy-import";
import { StrategyRuleBuilder } from "@/features/strategy-settings/strategy-rule-builder";
import type {
	StrategyFormApi,
	StrategyQuery,
	StrategyRuleField,
	StrategyRuleVariables,
} from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyFormValues,
} from "@/features/strategy-settings/utils";

interface StrategyConditionsProps {
	// Extra header actions, such as removing an optional rule.
	actions?: ReactNode;
	disabled: boolean;
	// Whether the rule is an exit rule, which may read position variables.
	exit: boolean;
	field: StrategyRuleField;
	form: StrategyFormApi;
	title: string;
	variables: StrategyRuleVariables;
}

// One rule of a strategy: its builder, import, and copyable expression. The
// builder edits the form field itself; only the header button and the
// expression follow the field.
export function StrategyConditions({
	actions,
	disabled,
	exit,
	field,
	form,
	title,
	variables,
}: StrategyConditionsProps) {
	const [importing, setImporting] = useState(false);
	// The query the uncontrolled builder mounted with; a replaced query
	// remounts it with the new one.
	const [mounted, setMounted] = useState(() => ({
		query: strategyFormValues(form.state.values)[field] ?? emptyStrategyQuery(),
		revision: 0,
	}));
	const setQuery = (query: StrategyQuery) => form.setFieldValue(field, query);
	const replace = (query: StrategyQuery) => {
		setQuery(query);
		setMounted((current) => ({ query, revision: current.revision + 1 }));
	};
	return (
		<Stack gap={4}>
			<Group justify="space-between">
				<Text fw={500} size="sm">
					{title}
				</Text>
				<Group gap={4}>
					<StrategyConditionsReplaceButton
						disabled={disabled}
						field={field}
						form={form}
						onClear={() => replace(emptyStrategyQuery())}
						onImport={() => setImporting(true)}
					/>
					{actions}
				</Group>
			</Group>
			<StrategyRuleBuilder
				defaultQuery={mounted.query}
				disabled={disabled}
				key={mounted.revision}
				onQueryChange={setQuery}
				variables={variables}
			/>
			<StrategyExpressionPreview field={field} form={form} title={title} />
			<StrategyImport
				exit={exit}
				onClose={() => setImporting(false)}
				onImport={replace}
				opened={importing}
			/>
		</Stack>
	);
}
