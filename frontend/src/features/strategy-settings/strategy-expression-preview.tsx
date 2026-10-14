import { Code, Text, Tooltip, UnstyledButton } from "@mantine/core";
import { useClipboard } from "@mantine/hooks";
import { useSelector } from "@tanstack/react-form";

import type {
	StrategyFormApi,
	StrategyRuleField,
} from "@/features/strategy-settings/types";
import {
	strategyExpression,
	strategyFormValues,
	strategyQueryComplete,
} from "@/features/strategy-settings/utils";

interface StrategyExpressionPreviewProps {
	field: StrategyRuleField;
	form: StrategyFormApi;
	title: string;
}

// The rule as the CEL expression it saves, copied by a tap, or what is
// missing before it can be saved. Serialized only when the rule changes.
export function StrategyExpressionPreview({
	field,
	form,
	title,
}: StrategyExpressionPreviewProps) {
	const query = useSelector(
		form.store,
		({ values }) => strategyFormValues(values)[field],
	);
	const expression =
		query && strategyQueryComplete(query) ? strategyExpression(query) : "";
	const clipboard = useClipboard({ timeout: 1500 });
	if (expression === "") {
		return (
			<Text c="dimmed" size="xs">
				Add at least one condition and fill every value.
			</Text>
		);
	}
	return (
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
	);
}
