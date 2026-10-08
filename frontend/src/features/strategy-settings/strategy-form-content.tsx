import {
	Alert,
	Button,
	Checkbox,
	Group,
	NumberInput,
	Stack,
	Text,
	Textarea,
	TextInput,
} from "@mantine/core";
import { useEffect, useState } from "react";
import type { StrategyUpdate, StrategyVariable } from "@/api/generated/models";
import { StrategyConditions } from "@/features/strategy-settings/strategy-conditions";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyExpression,
	strategyQueryComplete,
	withoutPositionVariables,
} from "@/features/strategy-settings/utils";

interface StrategyFormContentProps {
	draft: StrategyDraft;
	isSaving: boolean;
	onCancel(): void;
	// Reports whether the fields differ from the draft, so the page can
	// warn before another strategy replaces them.
	onDirtyChange(dirty: boolean): void;
	onSubmit(input: StrategyUpdate): void;
	variables: readonly StrategyVariable[];
}

// The fields of an opened strategy form; remounted for every draft.
export function StrategyFormContent({
	draft,
	isSaving,
	onCancel,
	onDirtyChange,
	onSubmit,
	variables,
}: StrategyFormContentProps) {
	const [name, setName] = useState(draft.name);
	const [message, setMessage] = useState(draft.message);
	const [query, setQuery] = useState(draft.query);
	const [exitQuery, setExitQuery] = useState(draft.exitQuery);
	const [accumulate, setAccumulate] = useState(draft.accumulate);
	const [maxBuys, setMaxBuys] = useState(draft.maxBuys);
	const exitExpression = exitQuery ? strategyExpression(exitQuery) : "";
	// Only a strategy with an exit rule accumulates.
	const accumulates = exitQuery !== undefined && accumulate;
	// Max buys bound trades that can hold several buys.
	const buysMany = exitQuery === undefined || accumulates;
	const input: StrategyUpdate = {
		accumulate: accumulates,
		exit_expression: exitExpression,
		expression: strategyExpression(query),
		max_buys: buysMany ? maxBuys : 0,
		message: message.trim(),
		name: name.trim(),
	};
	const named = input.name !== "";
	const complete =
		strategyQueryComplete(query) &&
		(exitQuery === undefined || strategyQueryComplete(exitQuery));
	const dirty =
		name !== draft.name ||
		message !== draft.message ||
		input.expression !== strategyExpression(draft.query) ||
		exitExpression !==
			(draft.exitQuery ? strategyExpression(draft.exitQuery) : "") ||
		input.accumulate !== draft.accumulate ||
		input.max_buys !== draft.maxBuys;
	useEffect(() => onDirtyChange(dirty), [dirty, onDirtyChange]);
	const entryVariables = withoutPositionVariables(variables);
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
			<Textarea
				autosize
				description="Sent in Telegram after the strategy name, coin, and the buy or sell instead of the rule and its values. Leave empty for the default text."
				disabled={isSaving}
				label="Message"
				maxLength={1000}
				minRows={2}
				onChange={(event) => setMessage(event.currentTarget.value)}
				value={message}
			/>
			{draft.incomplete ? (
				<Alert color="yellow" variant="light">
					Part of a stored rule cannot be shown here. Saving keeps only the
					conditions below.
				</Alert>
			) : null}
			<StrategyConditions
				disabled={isSaving}
				exit={false}
				onChange={setQuery}
				query={query}
				title="Entry"
				variables={entryVariables}
			/>
			{exitQuery ? (
				<StrategyConditions
					actions={
						<Button
							color="red"
							disabled={isSaving}
							onClick={() => setExitQuery(undefined)}
							size="compact-sm"
							variant="subtle"
						>
							Remove
						</Button>
					}
					disabled={isSaving}
					exit
					onChange={setExitQuery}
					query={exitQuery}
					title="Exit"
					variables={variables}
				/>
			) : (
				<Stack gap={4}>
					<Group justify="space-between">
						<Text fw={500} size="sm">
							Exit
						</Text>
						<Button
							disabled={isSaving}
							onClick={() => setExitQuery(emptyStrategyQuery())}
							size="compact-sm"
							variant="subtle"
						>
							Add exit rule
						</Button>
					</Group>
					<Text c="dimmed" size="xs">
						Without an exit rule the strategy never sells and buys every time
						its entry turns true.
					</Text>
				</Stack>
			)}
			{exitQuery ? (
				<Checkbox
					checked={accumulate}
					description="Each time the entry turns true during a trade, buy again; the exit sells every buy."
					disabled={isSaving}
					label="Accumulate"
					onChange={(event) => setAccumulate(event.currentTarget.checked)}
				/>
			) : null}
			{buysMany ? (
				<NumberInput
					allowDecimal={false}
					allowNegative={false}
					description="Most buys of one trade; 0 for no limit."
					disabled={isSaving}
					label="Max buys"
					max={1000}
					min={0}
					onChange={(value) =>
						setMaxBuys(typeof value === "number" ? value : 0)
					}
					value={maxBuys}
				/>
			) : null}
			{named ? null : (
				<Text c="dimmed" size="xs">
					Enter a name to save the strategy.
				</Text>
			)}
			<Group justify="flex-end">
				<Button disabled={isSaving} onClick={onCancel} variant="default">
					Cancel
				</Button>
				<Button
					disabled={!named || !complete}
					loading={isSaving}
					onClick={() => onSubmit(input)}
				>
					Save
				</Button>
			</Group>
		</Stack>
	);
}
