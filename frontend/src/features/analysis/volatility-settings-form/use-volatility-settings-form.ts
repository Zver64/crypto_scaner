import { useForm } from "@mantine/form";
import type { SettingsFormProps } from "@/components/settings-form/types";
import { applicationConfig } from "@/config";
import type { VolatilityFormField } from "@/features/analysis/volatility-form/types";
import { buildVolatilityInputs } from "@/features/analysis/volatility-form/utils";
import type {
	UseVolatilitySettingsFormOptions,
	VolatilitySettingsDraft,
} from "@/features/analysis/volatility-settings-form/types";
import {
	settingsFromValidDraft,
	validateVolatilitySettings,
} from "@/features/analysis/volatility-settings-form/utils";

export function useVolatilitySettingsForm({
	disabled,
	initialSettings,
	onCommit,
}: UseVolatilitySettingsFormOptions): SettingsFormProps {
	const presets = applicationConfig.volatility;
	const form = useForm<VolatilitySettingsDraft>({
		initialValues: initialSettings,
		mode: "controlled",
		validate: validateVolatilitySettings,
		validateInputOnChange: true,
	});
	const draftSettings = settingsFromValidDraft(form.values);

	function field(name: keyof VolatilitySettingsDraft): VolatilityFormField {
		return {
			error: form.errors[name],
			id: form.key(name),
			onChange: (value) => form.setFieldValue(name, value),
			value: form.values[name],
		};
	}

	return {
		groups: [
			{
				id: "daily",
				title: "Daily Volatility",
				inputs: buildVolatilityInputs({
					period: field("period"),
					percentile: field("percentile"),
					periodPresets: presets.days.periodPresets,
					percentilePresets: presets.percentilePresets,
					size: "md",
					unit: "days",
				}),
			},
			{
				id: "hourly",
				title: "Hourly Volatility",
				inputs: buildVolatilityInputs({
					period: field("hourlyPeriod"),
					percentile: field("hourlyPercentile"),
					periodPresets: presets.hours.periodPresets,
					percentilePresets: presets.percentilePresets,
					size: "md",
					unit: "hours",
				}),
			},
		],
		disabled: disabled || !draftSettings,
		onSubmit: (event) => {
			event.preventDefault();
			if (disabled || !draftSettings) return;
			onCommit(draftSettings);
		},
		submitLabel: "Apply settings",
	};
}
