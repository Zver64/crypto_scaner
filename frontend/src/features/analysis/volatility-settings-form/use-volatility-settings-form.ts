import { useSelector } from "@tanstack/react-form";
import { useId } from "react";

import { useAppForm } from "@/components/form/use-app-form";
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
	loading,
	onCommit,
}: UseVolatilitySettingsFormOptions): SettingsFormProps {
	const presets = applicationConfig.volatility;
	const id = useId();
	// The inputs may hold strings while being typed.
	const defaultValues: VolatilitySettingsDraft = initialSettings;
	const form = useAppForm({ defaultValues });
	// The fields and the Apply settings button follow the values as they are
	// typed; the button stays disabled while they are invalid.
	const values = useSelector(form.store, (state) => state.values);
	const errors = validateVolatilitySettings(values);
	const draftSettings = settingsFromValidDraft(values);

	function field(name: keyof VolatilitySettingsDraft): VolatilityFormField {
		return {
			error: errors[name],
			id: `${id}-${name}`,
			onChange: (value) => form.setFieldValue(name, value),
			value: values[name],
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
					unit: "hours",
				}),
			},
		],
		disabled: disabled || !draftSettings,
		loading,
		onSubmit: (event) => {
			event.preventDefault();
			if (disabled || !draftSettings) return;
			onCommit(draftSettings);
		},
		submitLabel: "Apply settings",
	};
}
