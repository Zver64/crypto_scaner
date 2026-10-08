import {
	Button,
	Code,
	Group,
	Stack,
	Text,
	Tooltip,
	UnstyledButton,
} from "@mantine/core";
import { useClipboard } from "@mantine/hooks";
import { type ReactNode, useState } from "react";
import type { StrategyVariable } from "@/api/generated/models";
import { StrategyImport } from "@/features/strategy-settings/strategy-import";
import { StrategyRuleBuilder } from "@/features/strategy-settings/strategy-rule-builder";
import type { StrategyQuery } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyExpression,
	strategyQueryComplete,
} from "@/features/strategy-settings/utils";

interface StrategyConditionsProps {
	// Extra header actions, such as removing an optional rule.
	actions?: ReactNode;
	disabled: boolean;
	// Whether the rule is an exit rule, which may read position variables.
	exit: boolean;
	onChange(query: StrategyQuery): void;
	query: StrategyQuery;
	title: string;
	variables: readonly StrategyVariable[];
}

// One rule of a strategy: its builder, import, and copyable expression.
export function StrategyConditions({
	actions,
	disabled,
	exit,
	onChange,
	query,
	title,
	variables,
}: StrategyConditionsProps) {
	const [importing, setImporting] = useState(false);
	const clipboard = useClipboard({ timeout: 1500 });
	const expression = strategyExpression(query);
	return (
		<Stack gap={4}>
			<Group justify="space-between">
				<Text fw={500} size="sm">
					{title}
				</Text>
				<Group gap={4}>
					{/* An empty builder imports an expression; conditions are
					    cleared first, so an import never replaces them. */}
					{query.rules.length === 0 ? (
						<Button
							disabled={disabled}
							onClick={() => setImporting(true)}
							size="compact-sm"
							variant="subtle"
						>
							Import
						</Button>
					) : (
						<Button
							color="red"
							disabled={disabled}
							onClick={() => onChange(emptyStrategyQuery())}
							size="compact-sm"
							variant="subtle"
						>
							Clear
						</Button>
					)}
					{actions}
				</Group>
			</Group>
			<StrategyRuleBuilder
				disabled={disabled}
				onChange={onChange}
				query={query}
				variables={variables}
			/>
			{strategyQueryComplete(query) ? (
				<>
					<Tooltip label="Copied" opened={clipboard.copied}>
						<UnstyledButton
							aria-label={`Copy the ${title.toLowerCase()}`}
							onClick={() => clipboard.copy(expression)}
						>
							<Code
								block
								style={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}
							>
								{expression}
							</Code>
						</UnstyledButton>
					</Tooltip>
					<Text c={clipboard.error ? "red" : "dimmed"} size="xs">
						{clipboard.error
							? "The expression could not be copied."
							: "Tap the expression to copy it."}
					</Text>
				</>
			) : (
				<Text c="dimmed" size="xs">
					Add at least one condition and fill every value.
				</Text>
			)}
			<StrategyImport
				exit={exit}
				onClose={() => setImporting(false)}
				onImport={onChange}
				opened={importing}
			/>
		</Stack>
	);
}
