import { Group, NumberInput, TagsInput } from "@mantine/core";
import type { ScaleDraft } from "@/features/scanner-settings/types";
import { parseLevels } from "@/features/scanner-settings/utils";

interface ScannerIndicatorScaleFieldsProps {
	onChange(value: ScaleDraft): void;
	value: ScaleDraft;
}

export function ScannerIndicatorScaleFields({
	onChange,
	value,
}: ScannerIndicatorScaleFieldsProps) {
	return (
		<>
			<Group grow>
				<NumberInput
					label="Scale min"
					onChange={(scaleMin) => onChange({ ...value, scaleMin })}
					placeholder="auto"
					value={value.scaleMin}
				/>
				<NumberInput
					label="Scale max"
					onChange={(scaleMax) => onChange({ ...value, scaleMax })}
					placeholder="auto"
					value={value.scaleMax}
				/>
			</Group>
			<TagsInput
				error={parseLevels(value.levels) ? undefined : "Levels must be numbers"}
				label="Levels"
				onChange={(levels) => onChange({ ...value, levels })}
				placeholder="For example 30, 70"
				value={value.levels}
			/>
		</>
	);
}
