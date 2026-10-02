import {
	Alert,
	Button,
	Code,
	Group,
	Stack,
	Text,
	Textarea,
	TextInput,
	Tooltip,
	UnstyledButton,
} from "@mantine/core";
import { useClipboard } from "@mantine/hooks";
import { useState } from "react";
import type { StrategyUpdate, StrategyVariable } from "@/api/generated/models";
import { StrategyImport } from "@/features/strategy-settings/strategy-import";
import { StrategyRuleBuilder } from "@/features/strategy-settings/strategy-rule-builder";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyExpression,
	strategyQueryComplete,
} from "@/features/strategy-settings/utils";

interface StrategyFormContentProps {
	draft: StrategyDraft;
	isSaving: boolean;
	onCancel(): void;
	onSubmit(input: StrategyUpdate): void;
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
	const [message, setMessage] = useState(draft.message);
	const [query, setQuery] = useState(draft.query);
	const [importing, setImporting] = useState(false);
	const clipboard = useClipboard({ timeout: 1500 });
	const empty = query.rules.length === 0;
	const expression = strategyExpression(query);
	const named = name.trim() !== "";
	const conditionsComplete = strategyQueryComplete(query);
	return (
		<Stack gap="md">
			<TextInput
				disabled={isSaving}
				label="Name"
				maxLength={64}
				withAsterisk
				onChange={(event) => setName(event.currentTarget.value)}
				value={name}
			/>
			{draft.incomplete ? (
				<Alert color="yellow" variant="light">
					Part of the stored expression cannot be shown here. Saving keeps only
					the conditions below.
				</Alert>
			) : null}
			<Stack gap={4}>
				<Group justify="space-between">
					<Text fw={500} size="sm">
						Conditions
					</Text>
					{/* An empty builder imports an expression; conditions are
					    cleared first, so an import never replaces them. */}
					{empty ? (
						<Button
							disabled={isSaving}
							onClick={() => setImporting(true)}
							size="compact-sm"
							variant="subtle"
						>
							Import
						</Button>
					) : (
						<Button
							color="red"
							disabled={isSaving}
							onClick={() => setQuery(emptyStrategyQuery())}
							size="compact-sm"
							variant="subtle"
						>
							Clear
						</Button>
					)}
				</Group>
				<StrategyRuleBuilder
					disabled={isSaving}
					onChange={setQuery}
					query={query}
					variables={variables}
				/>
			</Stack>
			{conditionsComplete ? (
				<Stack gap={4}>
					<Tooltip label="Copied" opened={clipboard.copied}>
						<UnstyledButton
							aria-label="Copy the expression"
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
				</Stack>
			) : (
				<Text c="dimmed" size="xs">
					Add at least one condition and fill every value.
				</Text>
			)}
			<Textarea
				autosize
				description="Sent in Telegram after the strategy name and coin instead of the expression and its values. Leave empty for the default text."
				disabled={isSaving}
				label="Message"
				maxLength={1000}
				minRows={2}
				onChange={(event) => setMessage(event.currentTarget.value)}
				value={message}
			/>
			{named ? null : (
				<Text c="dimmed" size="xs">
					Enter a name to save the strategy.
				</Text>
			)}
			<StrategyImport
				onClose={() => setImporting(false)}
				onImport={setQuery}
				opened={importing}
			/>
			<Group justify="flex-end">
				<Button disabled={isSaving} onClick={onCancel} variant="default">
					Cancel
				</Button>
				<Button
					disabled={!named || !conditionsComplete}
					loading={isSaving}
					onClick={() =>
						onSubmit({ expression, message: message.trim(), name: name.trim() })
					}
				>
					Save
				</Button>
			</Group>
		</Stack>
	);
}
