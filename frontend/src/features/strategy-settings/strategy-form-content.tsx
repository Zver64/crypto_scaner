import { Alert, Stack } from "@mantine/core";

import { strategyNameSchema } from "@/features/strategy-settings/schemas";
import { StrategyCoinFields } from "@/features/strategy-settings/strategy-coin-fields";
import { StrategyConditions } from "@/features/strategy-settings/strategy-conditions";
import { StrategyDirectionControl } from "@/features/strategy-settings/strategy-direction-control";
import { StrategyExitConditions } from "@/features/strategy-settings/strategy-exit-conditions";
import { StrategyFormFooter } from "@/features/strategy-settings/strategy-form-footer";
import { StrategyMarketCapFields } from "@/features/strategy-settings/strategy-market-cap-fields";
import { StrategyPriceInput } from "@/features/strategy-settings/strategy-price-input";
import { StrategySignalEvaluationFields } from "@/features/strategy-settings/strategy-signal-evaluation-fields";
import type {
	StrategyDraft,
	StrategyFormApi,
	StrategyVariableSets,
} from "@/features/strategy-settings/types";
import { strategyKind } from "@/features/strategy-settings/utils";

interface StrategyFormContentProps {
	draft: StrategyDraft;
	// Holds the fields of the draft; the page owns it, so it can ask before
	// another strategy replaces unsaved changes.
	form: StrategyFormApi;
	isSaving: boolean;
	onCancel(): void;
	variables: StrategyVariableSets;
}

// The fields of an opened strategy form; remounted for every draft. Not an
// HTML form: Enter in a rule's input must not save the strategy. Each
// field and each component that shows values subscribes to its own part of
// the form, so a keystroke renders only those.
export function StrategyFormContent({
	draft,
	form,
	isSaving,
	onCancel,
	variables,
}: StrategyFormContentProps) {
	return (
		<Stack gap="md">
			<form.AppField
				name="name"
				validators={{
					// Checked from the start, since Save stays disabled without a name.
					onChange: strategyNameSchema,
					onMount: strategyNameSchema,
				}}
			>
				{(field) => (
					<field.TextField
						disabled={isSaving}
						label="Name"
						maxLength={64}
						withAsterisk
					/>
				)}
			</form.AppField>
			<form.AppField name="message">
				{(field) => (
					<field.TextareaField
						autosize
						description="Sent in Telegram under the alert title, above the coins. Leave empty for none."
						disabled={isSaving}
						label="Message"
						maxLength={1000}
						minRows={2}
					/>
				)}
			</form.AppField>
			{/* A saved strategy keeps its direction. */}
			<StrategyDirectionControl
				disabled={isSaving || draft.id !== undefined}
				form={form}
				kind={strategyKind(draft)}
			/>
			<StrategyCoinFields
				disabled={isSaving}
				form={form}
				signal={draft.signal}
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
				field="query"
				form={form}
				title="Entry"
				variables={variables.entry}
			/>
			{/* A signal trades nothing, so it has no exit or price fields;
			    only a signal has a target ratio, a window, and a market cap
			    range. */}
			{draft.signal ? (
				<>
					<StrategySignalEvaluationFields disabled={isSaving} form={form} />
					<StrategyMarketCapFields disabled={isSaving} form={form} />
				</>
			) : (
				<>
					<StrategyExitConditions
						disabled={isSaving}
						form={form}
						variables={variables.exit}
					/>
					<StrategyPriceInput
						disabled={isSaving}
						field="takeProfit"
						form={form}
						label="Take profit"
						variables={variables.entry.variables}
					/>
					<StrategyPriceInput
						disabled={isSaving}
						field="stopLoss"
						form={form}
						label="Stop loss"
						variables={variables.entry.variables}
					/>
				</>
			)}
			<StrategyFormFooter
				form={form}
				isSaving={isSaving}
				onCancel={onCancel}
				signal={draft.signal}
			/>
		</Stack>
	);
}
