import { Group, NumberInput, TagsInput } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";

import type { ScaleDraft } from "@/features/scanner-settings/types";

interface ScannerIndicatorScaleFieldsProps {
	// The indicator form and the scale editor both pass their form, which
	// validates the levels with levelsError on every change.
	form: UseFormReturnType<ScaleDraft>;
}

export function ScannerIndicatorScaleFields({
	form,
}: ScannerIndicatorScaleFieldsProps) {
	return (
		<>
			<Group grow>
				<NumberInput
					label="Scale min"
					placeholder="auto"
					{...form.getInputProps("scaleMin")}
				/>
				<NumberInput
					label="Scale max"
					placeholder="auto"
					{...form.getInputProps("scaleMax")}
				/>
			</Group>
			<TagsInput
				label="Levels"
				placeholder="For example 30, 70"
				{...form.getInputProps("levels")}
			/>
		</>
	);
}
