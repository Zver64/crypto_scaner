import { Button, Group, Stack } from "@mantine/core";
import { useForm } from "@mantine/form";
import type { ScannerIndicatorScale } from "@/api/generated/models";
import { ScannerIndicatorScaleFields } from "@/features/scanner-settings/scanner-indicator-scale-fields";
import {
	indicatorScale,
	levelsError,
	scaleDraft,
} from "@/features/scanner-settings/utils";

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
	// Controlled, because the levels error and the Save button follow the
	// values as they change.
	const form = useForm({
		initialValues: scaleDraft(scale),
		mode: "controlled",
		validate: { levels: levelsError },
		validateInputOnChange: true,
	});
	return (
		<form
			onSubmit={form.onSubmit((values) => {
				const next = indicatorScale(values);
				if (next) onSave(next);
			})}
		>
			<Stack gap="xs">
				<ScannerIndicatorScaleFields form={form} />
				<Group gap="xs" justify="flex-end">
					<Button onClick={onCancel} size="compact-xs" variant="subtle">
						Cancel
					</Button>
					<Button
						disabled={disabled || !indicatorScale(form.values)}
						size="compact-xs"
						type="submit"
					>
						Save
					</Button>
				</Group>
			</Stack>
		</form>
	);
}
