import { Button, Group, Stack } from "@mantine/core";
import { type FormEvent, useState } from "react";
import type { ScannerIndicatorScale } from "@/api/generated/models";
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
	const [draft, setDraft] = useState(() => scaleDraft(scale));
	const next = indicatorScale(draft);
	const submit = (event: FormEvent) => {
		event.preventDefault();
		if (next) onSave(next);
	};
	return (
		<Stack component="form" gap="xs" onSubmit={submit}>
			<ScannerIndicatorScaleFields onChange={setDraft} value={draft} />
			<Group gap="xs" justify="flex-end">
				<Button onClick={onCancel} size="compact-xs" variant="subtle">
					Cancel
				</Button>
				<Button disabled={disabled || !next} size="compact-xs" type="submit">
					Save
				</Button>
			</Group>
		</Stack>
	);
}
