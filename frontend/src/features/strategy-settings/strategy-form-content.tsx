import {
	Alert,
	Button,
	Group,
	Stack,
	Text,
	Textarea,
	TextInput,
} from "@mantine/core";
import { useEffect, useState } from "react";
import type { StrategyUpdate, StrategyVariable } from "@/api/generated/models";
import { StrategyConditions } from "@/features/strategy-settings/strategy-conditions";
import { StrategyMarketCapFields } from "@/features/strategy-settings/strategy-market-cap-fields";
import { StrategyPriceInput } from "@/features/strategy-settings/strategy-price-input";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	marketCapErrors,
	marketCapUsd,
	strategyBuysLabel,
	strategyExits,
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
	const [takeProfit, setTakeProfit] = useState(draft.takeProfit);
	const [stopLoss, setStopLoss] = useState(draft.stopLoss);
	const [minMarketCap, setMinMarketCap] = useState<number | string>(
		draft.minMarketCap,
	);
	const [maxMarketCap, setMaxMarketCap] = useState<number | string>(
		draft.maxMarketCap,
	);
	const exitExpression = exitQuery ? strategyExpression(exitQuery) : "";
	const input: StrategyUpdate = {
		exit_expression: exitExpression,
		expression: strategyExpression(query),
		max_market_cap_usd: marketCapUsd(maxMarketCap),
		message: message.trim(),
		min_market_cap_usd: marketCapUsd(minMarketCap),
		name: name.trim(),
		stop_loss_expression: stopLoss.trim(),
		take_profit_expression: takeProfit.trim(),
	};
	const named = input.name !== "";
	const boundErrors = marketCapErrors(
		input.min_market_cap_usd,
		input.max_market_cap_usd,
	);
	const marketCapValid = !boundErrors.minimum && !boundErrors.maximum;
	const complete =
		strategyQueryComplete(query) &&
		(exitQuery === undefined || strategyQueryComplete(exitQuery));
	const dirty =
		name !== draft.name ||
		message !== draft.message ||
		input.expression !== strategyExpression(draft.query) ||
		exitExpression !==
			(draft.exitQuery ? strategyExpression(draft.exitQuery) : "") ||
		input.take_profit_expression !== draft.takeProfit ||
		input.stop_loss_expression !== draft.stopLoss ||
		input.min_market_cap_usd !== marketCapUsd(draft.minMarketCap) ||
		input.max_market_cap_usd !== marketCapUsd(draft.maxMarketCap);
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
				</Stack>
			)}
			<StrategyPriceInput
				description="Sells the trade when a candle reaches this price, fixed when the entry signals, such as h_close * 1.05. Leave empty for none."
				disabled={isSaving}
				label="Take profit"
				onChange={setTakeProfit}
				value={takeProfit}
			/>
			<StrategyPriceInput
				description="Sells the trade when a candle falls to this price, fixed when the entry signals, such as h_close - 2 * h_atr_14. Leave empty for none."
				disabled={isSaving}
				label="Stop loss"
				onChange={setStopLoss}
				value={stopLoss}
			/>
			<StrategyMarketCapFields
				disabled={isSaving}
				errors={boundErrors}
				maximum={maxMarketCap}
				minimum={minMarketCap}
				onMaximumChange={setMaxMarketCap}
				onMinimumChange={setMinMarketCap}
			/>
			<Text c="dimmed" size="xs">
				{strategyBuysLabel(input)}.{" "}
				{strategyExits(input)
					? "Entry signals during a trade buy nothing."
					: "Without an exit rule, take profit, or stop loss, every time the entry turns true buys."}
			</Text>
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
					disabled={!named || !complete || !marketCapValid}
					loading={isSaving}
					onClick={() => onSubmit(input)}
				>
					Save
				</Button>
			</Group>
		</Stack>
	);
}
