import { Group } from "@mantine/core";

import { withFieldGroup } from "@/components/form";
import { emptyScaleDraft } from "@/features/scanner-settings/constants";
import { levelsSchema } from "@/features/scanner-settings/schemas";

// The pane scale of the indicator form and the scale editor. The levels are
// validated on every change.
export const ScannerIndicatorScaleFields = withFieldGroup({
	defaultValues: emptyScaleDraft,
	render: function ScannerIndicatorScaleFields({ group }) {
		return (
			<>
				<Group grow>
					<group.AppField name="scaleMin">
						{(field) => (
							<field.NumberField label="Scale min" placeholder="auto" />
						)}
					</group.AppField>
					<group.AppField name="scaleMax">
						{(field) => (
							<field.NumberField label="Scale max" placeholder="auto" />
						)}
					</group.AppField>
				</Group>
				<group.AppField name="levels" validators={{ onChange: levelsSchema }}>
					{(field) => (
						<field.TagsField label="Levels" placeholder="For example 30, 70" />
					)}
				</group.AppField>
			</>
		);
	},
});
