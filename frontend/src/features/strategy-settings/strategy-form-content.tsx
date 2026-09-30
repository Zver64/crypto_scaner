import { Button, Code, Group, Stack, Text, TextInput } from "@mantine/core";
import { useState } from "react";
import type { StrategyVariable } from "@/api/generated/models";
import { StrategyRuleBuilder } from "@/features/strategy-settings/strategy-rule-builder";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	strategyExpression,
	strategyQueryComplete,
} from "@/features/strategy-settings/utils";

interface StrategyFormContentProps {
	draft: StrategyDraft;
	isSaving: boolean;
	onCancel(): void;
	onSubmit(name: string, expression: string): void;
	variables: readonly StrategyVariable[];
}

// The fields of an opened strategy form; remounted for every draft.
export function StrategyFormContent({
	draft,
	isSaving,
	onCancel,
	onSubmit,
	variables,
}: StrategyFormContentProps) {
	const [name, setName] = useState(draft.name);
	const [query, setQuery] = useState(draft.query);
	const expression = strategyExpression(query);
	const complete = name.trim() !== "" && strategyQueryComplete(query);
	return (
		<Stack gap="md">
			<TextInput
				disabled={isSaving}
				label="Name"
				maxLength={64}
				onChange={(event) => setName(event.currentTarget.value)}
				value={name}
			/>
			<Stack gap={4}>
				<Text fw={500} size="sm">
					Conditions
				</Text>
				<StrategyRuleBuilder
					disabled={isSaving}
					onChange={setQuery}
					query={query}
					variables={variables}
				/>
			</Stack>
			{complete ? (
				<Code block>{expression}</Code>
			) : (
				<Text c="dimmed" size="xs">
					Add at least one condition and fill every value.
				</Text>
			)}
			<Group justify="flex-end">
				<Button disabled={isSaving} onClick={onCancel} variant="default">
					Cancel
				</Button>
				<Button
					disabled={!complete}
					loading={isSaving}
					onClick={() => onSubmit(name.trim(), expression)}
				>
					Save
				</Button>
			</Group>
		</Stack>
	);
}
