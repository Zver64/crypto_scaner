import { Fieldset, Stack } from "@mantine/core";
import type { ReactNode } from "react";

import { withFieldGroup } from "@/components/form";
import { applicationConfig } from "@/config";
import {
	type AnalysisUnit,
	analysisCriteriaConstraints,
} from "@/features/analysis/criteria";
import { presetOptions } from "@/features/analysis/volatility-form/utils";
import type { VolatilitySettingsDraft } from "@/features/analysis/volatility-settings-form/types";

// The fields the group binds, as the inputs emit them; only the keys matter.
const volatilityDraft: Pick<VolatilitySettingsDraft, "percentile" | "period"> =
	{
		percentile: "",
		period: "",
	};

interface VolatilityFieldsProps {
	// Fields of the criterion's own, shown after the shared ones.
	children?: ReactNode;
	legend: string;
	unit: AnalysisUnit;
}

// One volatility criterion: its period and percentile, followed by any
// fields of its own, with the unit's presets. The percentile takes only its
// presets. The form validates them.
export const VolatilityFields = withFieldGroup({
	defaultValues: volatilityDraft,
	// Only types the props; every caller passes them.
	props: {} as VolatilityFieldsProps,
	render: function VolatilityFields({ children, group, legend, unit }) {
		const { percentilePresets } = applicationConfig.volatility;
		const { periodPresets } = applicationConfig.volatility[unit];
		return (
			<Fieldset legend={legend}>
				<Stack gap="xs">
					<group.AppField name="period">
						{(field) => (
							<field.NumberField
								allowDecimal={false}
								label="Period"
								max={analysisCriteriaConstraints.period.maximum}
								min={analysisCriteriaConstraints.period.minimum}
								presets={presetOptions(periodPresets)}
							/>
						)}
					</group.AppField>
					<group.AppField name="percentile">
						{(field) => (
							<field.SliderField
								label="Percentile"
								marks={presetOptions(percentilePresets)}
								max={Math.max(...percentilePresets)}
								min={Math.min(...percentilePresets)}
								restrictToMarks
							/>
						)}
					</group.AppField>
					{children}
				</Stack>
			</Fieldset>
		);
	},
});
