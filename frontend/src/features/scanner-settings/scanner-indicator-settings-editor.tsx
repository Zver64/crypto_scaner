import { Button, Group, Stack } from "@mantine/core";

import type {
	ScannerIndicator,
	ScannerIndicatorUpdate,
} from "@/api/generated/models";
import { useAppForm } from "@/components/form";
import {
	periodOrder,
	scaleFields,
} from "@/features/scanner-settings/constants";
import { ScannerIndicatorPeriodsField } from "@/features/scanner-settings/scanner-indicator-periods-field";
import { ScannerIndicatorScaleFields } from "@/features/scanner-settings/scanner-indicator-scale-fields";
import {
	scannerIndicatorSettingsDraft,
	scannerIndicatorUpdate,
} from "@/features/scanner-settings/utils";

interface ScannerIndicatorSettingsEditorProps {
	disabled: boolean;
	indicator: ScannerIndicator;
	onCancel(): void;
	onSave(update: ScannerIndicatorUpdate): void;
}

// Changes the periods of a configured indicator and, for a pane indicator,
// its value axis and levels.
export function ScannerIndicatorSettingsEditor({
	disabled,
	indicator,
	onCancel,
	onSave,
}: ScannerIndicatorSettingsEditorProps) {
	const form = useAppForm({
		defaultValues: scannerIndicatorSettingsDraft(indicator),
		onSubmit: ({ value }) => {
			const update = scannerIndicatorUpdate(value, indicator, periodOrder);
			if (update) onSave(update);
		},
	});
	return (
		<form
			onSubmit={(event) => {
				event.preventDefault();
				void form.handleSubmit();
			}}
		>
			<Stack gap="xs">
				<form.Field name="periods">
					{(field) => (
						<ScannerIndicatorPeriodsField
							onChange={field.handleChange}
							tableAllowed={indicator.outputs.length === 1}
							value={field.state.value}
						/>
					)}
				</form.Field>
				{indicator.placement === "pane" ? (
					<ScannerIndicatorScaleFields fields={scaleFields} form={form} />
				) : null}
				<Group gap="xs" justify="flex-end">
					<Button onClick={onCancel} size="compact-xs" variant="subtle">
						Cancel
					</Button>
					<form.AppForm>
						<form.SubmitButton disabled={disabled} size="compact-xs">
							Save
						</form.SubmitButton>
					</form.AppForm>
				</Group>
			</Stack>
		</form>
	);
}
