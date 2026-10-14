import { Alert, Stack, Textarea, TextInput } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import { StrategyConditions } from "@/features/strategy-settings/strategy-conditions";
import { StrategyDirectionControl } from "@/features/strategy-settings/strategy-direction-control";
import { StrategyExitConditions } from "@/features/strategy-settings/strategy-exit-conditions";
import { StrategyFormFooter } from "@/features/strategy-settings/strategy-form-footer";
import { StrategyMarketCapFields } from "@/features/strategy-settings/strategy-market-cap-fields";
import { StrategyPriceInput } from "@/features/strategy-settings/strategy-price-input";
import { StrategySignalEvaluationFields } from "@/features/strategy-settings/strategy-signal-evaluation-fields";
import type {
	StrategyDraft,
	StrategyFormValues,
	StrategyVariableSets,
} from "@/features/strategy-settings/types";
import { strategyKind } from "@/features/strategy-settings/utils";

interface StrategyFormContentProps {
	draft: StrategyDraft;
	// Holds the fields of the draft; the page owns it, so it can ask before
	// another strategy replaces unsaved changes.
	form: UseFormReturnType<StrategyFormValues>;
	isSaving: boolean;
	onCancel(): void;
	onSubmit(): void;
	variables: StrategyVariableSets;
}

// The fields of an opened strategy form; remounted for every draft. The
// inputs are uncontrolled, so most keystrokes render only the components that
// watch the edited field; a field turning dirty or clean re-renders the whole
// page that owns the form, without remounting the rule builders.
export function StrategyFormContent({
	draft,
	form,
	isSaving,
	onCancel,
	onSubmit,
	variables,
}: StrategyFormContentProps) {
	return (
		<Stack gap="md">
			<TextInput
				{...form.getInputProps("name")}
				disabled={isSaving}
				key={form.key("name")}
				label="Name"
				maxLength={64}
				withAsterisk
			/>
			<Textarea
				{...form.getInputProps("message")}
				autosize
				description="Sent in Telegram after the strategy name, coin, and the buy, sell, or signal instead of the rule and its values. Leave empty for the default text."
				disabled={isSaving}
				key={form.key("message")}
				label="Message"
				maxLength={1000}
				minRows={2}
			/>
			{/* A saved strategy keeps its direction. */}
			<StrategyDirectionControl
				disabled={isSaving || draft.id !== undefined}
				form={form}
				kind={strategyKind(draft)}
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
			    only a signal has a target ratio and a window. */}
			{draft.signal ? (
				<StrategySignalEvaluationFields disabled={isSaving} form={form} />
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
			<StrategyMarketCapFields disabled={isSaving} form={form} />
			<StrategyFormFooter
				form={form}
				isSaving={isSaving}
				onCancel={onCancel}
				onSubmit={onSubmit}
				signal={draft.signal}
			/>
		</Stack>
	);
}
