import { Button, Group, Text } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import { useState } from "react";
import { Direction } from "@/api/generated/models";
import { directionLabels } from "@/features/strategy-settings/constants";
import type { StrategyFormValues } from "@/features/strategy-settings/types";
import {
	strategyBuysLabel,
	strategyExits,
	strategyFormUpdate,
	strategyInput,
} from "@/features/strategy-settings/utils";

interface StrategyFormFooterProps {
	form: UseFormReturnType<StrategyFormValues>;
	isSaving: boolean;
	onCancel(): void;
	onSubmit(): void;
	// Whether the strategy only signals.
	signal: boolean;
}

// How the strategy buys, what is missing before it can be saved, and the
// form actions. The only part of the form that follows every field it
// summarizes; the message and the signal evaluation say nothing here.
export function StrategyFormFooter({
	form,
	isSaving,
	onCancel,
	onSubmit,
	signal,
}: StrategyFormFooterProps) {
	const [values, setValues] = useState(form.getValues);
	const sync = () => setValues(form.getValues());
	form.watch("name", sync);
	form.watch("direction", sync);
	form.watch("query", sync);
	form.watch("exitQuery", sync);
	form.watch("takeProfit", sync);
	form.watch("stopLoss", sync);
	form.watch("minMarketCap", sync);
	form.watch("maxMarketCap", sync);
	const input = strategyFormUpdate(values, signal);
	const named = input.name !== "";
	// Entries on coins outside a set range neither buy nor signal.
	const withinRange =
		input.min_market_cap_usd === null && input.max_market_cap_usd === null
			? ""
			: " on a coin within the market cap range";
	return (
		<>
			<Text c="dimmed" size="xs">
				{strategyBuysLabel(input)}.{" "}
				{signal
					? `Every time the entry turns true${withinRange} sends a ${directionLabels[input.direction].toLowerCase()} signal alert.`
					: strategyExits(input)
						? `Entry signals during a trade ${input.direction === Direction.short ? "short" : "buy"} nothing.`
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
					disabled={!strategyInput(values, signal)}
					loading={isSaving}
					onClick={onSubmit}
				>
					Save
				</Button>
			</Group>
		</>
	);
}
