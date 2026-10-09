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
import {
	Direction,
	type StrategyUpdate,
	type StrategyVariable,
} from "@/api/generated/models";
import { directionLabels } from "@/features/strategy-settings/constants";
import { StrategyConditions } from "@/features/strategy-settings/strategy-conditions";
import { StrategyDirectionControl } from "@/features/strategy-settings/strategy-direction-control";
import { StrategyMarketCapFields } from "@/features/strategy-settings/strategy-market-cap-fields";
import { StrategyPriceInput } from "@/features/strategy-settings/strategy-price-input";
import {
	parsePriceSetup,
	priceSetupChanged,
	priceSetupComplete,
	priceSetupExpression,
} from "@/features/strategy-settings/strategy-price-input/utils";
import { StrategySignalEvaluationFields } from "@/features/strategy-settings/strategy-signal-evaluation-fields";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	marketCapErrors,
	marketCapUsd,
	strategyBuysLabel,
	strategyExits,
	strategyExpression,
	strategyKind,
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
	const [direction, setDirection] = useState(draft.direction);
	const [message, setMessage] = useState(draft.message);
	const [query, setQuery] = useState(draft.query);
	const [exitQuery, setExitQuery] = useState(draft.exitQuery);
	const [takeProfit, setTakeProfit] = useState(() =>
		parsePriceSetup(draft.takeProfit),
	);
	const [stopLoss, setStopLoss] = useState(() =>
		parsePriceSetup(draft.stopLoss),
	);
	const [minMarketCap, setMinMarketCap] = useState<number | string>(
		draft.minMarketCap,
	);
	const [maxMarketCap, setMaxMarketCap] = useState<number | string>(
		draft.maxMarketCap,
	);
	const [targetRatio, setTargetRatio] = useState(draft.targetRatio);
	const [window, setWindow] = useState(draft.window);
	// A signal trades nothing, so it has no exit or price fields and sends
	// them empty; only a signal has a target ratio and a window.
	const trades = !draft.signal;
	const exitExpression = exitQuery ? strategyExpression(exitQuery) : "";
	const input: StrategyUpdate = {
		exit_expression: trades ? exitExpression : "",
		expression: strategyExpression(query),
		max_market_cap_usd: marketCapUsd(maxMarketCap),
		message: message.trim(),
		min_market_cap_usd: marketCapUsd(minMarketCap),
		name: name.trim(),
		signal: draft.signal,
		direction,
		stop_loss_expression: trades ? priceSetupExpression(stopLoss) : "",
		take_profit_expression: trades ? priceSetupExpression(takeProfit) : "",
		target_ratio: trades ? null : targetRatio,
		window: trades ? null : window,
	};
	const named = input.name !== "";
	const boundErrors = marketCapErrors(
		input.min_market_cap_usd,
		input.max_market_cap_usd,
	);
	const marketCapValid = !boundErrors.minimum && !boundErrors.maximum;
	// Entries on coins outside a set range neither buy nor signal.
	const withinRange =
		input.min_market_cap_usd === null && input.max_market_cap_usd === null
			? ""
			: " on a coin within the market cap range";
	const complete =
		strategyQueryComplete(query) &&
		(!trades ||
			((exitQuery === undefined || strategyQueryComplete(exitQuery)) &&
				priceSetupComplete(takeProfit) &&
				priceSetupComplete(stopLoss) &&
				// A short strategy always has a take profit and a stop loss.
				(direction !== Direction.short ||
					(input.take_profit_expression !== "" &&
						input.stop_loss_expression !== ""))));
	// A signal has no exit or price fields to differ.
	const dirty =
		name !== draft.name ||
		message !== draft.message ||
		input.direction !== draft.direction ||
		input.expression !== strategyExpression(draft.query) ||
		input.min_market_cap_usd !== marketCapUsd(draft.minMarketCap) ||
		input.max_market_cap_usd !== marketCapUsd(draft.maxMarketCap) ||
		(!trades &&
			(targetRatio !== draft.targetRatio || window !== draft.window)) ||
		(trades &&
			(exitExpression !==
				(draft.exitQuery ? strategyExpression(draft.exitQuery) : "") ||
				priceSetupChanged(takeProfit, parsePriceSetup(draft.takeProfit)) ||
				priceSetupChanged(stopLoss, parsePriceSetup(draft.stopLoss))));
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
				description="Sent in Telegram after the strategy name, coin, and the buy, sell, or signal instead of the rule and its values. Leave empty for the default text."
				disabled={isSaving}
				label="Message"
				maxLength={1000}
				minRows={2}
				onChange={(event) => setMessage(event.currentTarget.value)}
				value={message}
			/>
			{/* A saved strategy keeps its direction. */}
			<StrategyDirectionControl
				direction={direction}
				disabled={isSaving || draft.id !== undefined}
				kind={strategyKind(draft)}
				onChange={(next) => {
					setDirection(next);
					// Take profit and stop loss are written for one side.
					setTakeProfit({});
					setStopLoss({});
				}}
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
			{trades ? (
				<>
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
						disabled={isSaving}
						label="Take profit"
						onChange={setTakeProfit}
						value={takeProfit}
						variables={entryVariables}
					/>
					<StrategyPriceInput
						disabled={isSaving}
						label="Stop loss"
						onChange={setStopLoss}
						value={stopLoss}
						variables={entryVariables}
					/>
				</>
			) : (
				<StrategySignalEvaluationFields
					disabled={isSaving}
					onTargetRatioChange={setTargetRatio}
					onWindowChange={setWindow}
					targetRatio={targetRatio}
					window={window}
				/>
			)}
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
				{!trades
					? `Every time the entry turns true${withinRange} sends a ${directionLabels[direction].toLowerCase()} signal alert.`
					: strategyExits(input)
						? `Entry signals during a trade ${direction === Direction.short ? "short" : "buy"} nothing.`
						: `Without an exit rule, take profit, or stop loss, every time the entry turns true${withinRange} buys.`}
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
