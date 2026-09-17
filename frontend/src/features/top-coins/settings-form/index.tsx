import { Button, Paper, Stack, useMatches } from "@mantine/core";
import { useForm } from "@mantine/form";
import type { FormEvent } from "react";
import { applicationConfig } from "@/config";
import { VolatilityFieldset } from "@/features/analysis/volatility-form";
import type {
	TopCoinsSettingsDraft,
	TopCoinsSettingsFormProps,
} from "@/features/top-coins/settings-form/types";
import {
	settingsFromValidDraft,
	validateTopCoinsSettings,
} from "@/features/top-coins/settings-form/utils";

export function TopCoinsSettingsForm({
	disabled,
	initialSettings,
	onCommit,
}: TopCoinsSettingsFormProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const inputSize = "md";
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	const volatilityPresets = applicationConfig.volatility;
	const form = useForm<TopCoinsSettingsDraft>({
		initialValues: initialSettings,
		mode: "controlled",
		validate: validateTopCoinsSettings,
		validateInputOnChange: true,
	});
	const draftSettings = settingsFromValidDraft(form.values);

	const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!draftSettings) return;
		onCommit(draftSettings);
	};

	return (
		<Paper component="form" onSubmit={handleSubmit} p={paperPadding}>
			<Stack gap={contentSpacing}>
				<VolatilityFieldset
					percentile={{
						error: form.errors.percentile,
						id: form.key("percentile"),
						onChange: (value) => form.setFieldValue("percentile", value),
						value: form.values.percentile,
					}}
					percentilePresets={volatilityPresets.percentilePresets}
					period={{
						error: form.errors.period,
						id: form.key("period"),
						onChange: (value) => form.setFieldValue("period", value),
						value: form.values.period,
					}}
					periodPresets={volatilityPresets.days.periodPresets}
					size={inputSize}
					title="Daily Volatility"
					unit="days"
				/>
				<VolatilityFieldset
					percentile={{
						error: form.errors.hourlyPercentile,
						id: form.key("hourlyPercentile"),
						onChange: (value) => form.setFieldValue("hourlyPercentile", value),
						value: form.values.hourlyPercentile,
					}}
					percentilePresets={volatilityPresets.percentilePresets}
					period={{
						error: form.errors.hourlyPeriod,
						id: form.key("hourlyPeriod"),
						onChange: (value) => form.setFieldValue("hourlyPeriod", value),
						value: form.values.hourlyPeriod,
					}}
					periodPresets={volatilityPresets.hours.periodPresets}
					size={inputSize}
					title="Hourly Volatility"
					unit="hours"
				/>
				<Button
					disabled={disabled || !draftSettings}
					size={inputSize}
					type="submit"
				>
					Apply settings
				</Button>
			</Stack>
		</Paper>
	);
}
