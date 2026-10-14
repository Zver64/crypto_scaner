import { Button, Group, Text } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import { Direction } from "@/api/generated/models";
import { directionLabels } from "@/features/strategy-settings/constants";
import type { StrategyFormApi } from "@/features/strategy-settings/types";
import {
	strategyBuysLabel,
	strategyExits,
	strategyFormUpdate,
	strategyFormValues,
	strategyInput,
} from "@/features/strategy-settings/utils";

interface StrategyFormFooterProps {
	form: StrategyFormApi;
	isSaving: boolean;
	onCancel(): void;
	// Whether the strategy only signals.
	signal: boolean;
}

// How the strategy buys, and the form actions. The only part of the form
// that follows every field.
export function StrategyFormFooter({
	form,
	isSaving,
	onCancel,
	signal,
}: StrategyFormFooterProps) {
	const values = strategyFormValues(
		useSelector(form.store, (state) => state.values),
	);
	const input = strategyFormUpdate(values, signal);
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
			<Group justify="flex-end">
				<Button disabled={isSaving} onClick={onCancel} variant="default">
					Cancel
				</Button>
				{/* Rules and prices are not form fields, so the button also
				    stays disabled while strategyInput cannot build the update. */}
				<form.AppForm>
					<form.SubmitButton
						disabled={!strategyInput(values, signal)}
						loading={isSaving}
						onClick={() => void form.handleSubmit()}
					>
						Save
					</form.SubmitButton>
				</form.AppForm>
			</Group>
		</>
	);
}
