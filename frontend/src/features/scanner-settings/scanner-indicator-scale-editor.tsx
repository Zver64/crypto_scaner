import { Button, Group, Stack } from "@mantine/core";

import type { ScannerIndicatorScale } from "@/api/generated/models";
import { useAppForm } from "@/components/form";
import { scaleFields } from "@/features/scanner-settings/constants";
import { ScannerIndicatorScaleFields } from "@/features/scanner-settings/scanner-indicator-scale-fields";
import { indicatorScale, scaleDraft } from "@/features/scanner-settings/utils";

interface ScannerIndicatorScaleEditorProps {
	disabled: boolean;
	onCancel(): void;
	onSave(scale: ScannerIndicatorScale): void;
	scale: ScannerIndicatorScale;
}

// Changes the value axis and levels of a configured pane indicator.
export function ScannerIndicatorScaleEditor({
	disabled,
	onCancel,
	onSave,
	scale,
}: ScannerIndicatorScaleEditorProps) {
	const form = useAppForm({
		defaultValues: scaleDraft(scale),
		onSubmit: ({ value }) => {
			const next = indicatorScale(value);
			if (next) onSave(next);
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
				<ScannerIndicatorScaleFields fields={scaleFields} form={form} />
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
