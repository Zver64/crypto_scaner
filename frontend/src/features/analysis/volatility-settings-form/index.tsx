import {
	Paper,
	SimpleGrid,
	type SimpleGridProps,
	Stack,
	useMatches,
} from "@mantine/core";

import { useAppForm } from "@/components/form";
import { applicationConfig } from "@/config";
import { VolatilityFields } from "@/features/analysis/volatility-form";
import type {
	VolatilitySettings,
	VolatilitySettingsDraft,
} from "@/features/analysis/volatility-settings-form/types";
import {
	settingsFromValidDraft,
	validateVolatilitySettings,
} from "@/features/analysis/volatility-settings-form/utils";

interface VolatilitySettingsFormProps {
	// Grid columns of the fieldsets; one per row by default.
	columns?: SimpleGridProps["cols"];
	disabled: boolean;
	initialSettings: VolatilitySettings;
	loading?: boolean;
	onCommit(settings: VolatilitySettings): void;
}

export function VolatilitySettingsForm({
	columns = 1,
	disabled,
	initialSettings,
	loading,
	onCommit,
}: VolatilitySettingsFormProps) {
	const spacing = useMatches({ base: "xs", sm: "sm" });
	const padding = useMatches({ base: "xs", sm: "md" });
	const presets = applicationConfig.volatility;
	// The inputs may hold strings while being typed.
	const defaultValues: VolatilitySettingsDraft = initialSettings;
	const form = useAppForm({
		defaultValues,
		onSubmit: ({ value }) => {
			const settings = settingsFromValidDraft(value);
			if (settings) onCommit(settings);
		},
		validators: {
			onChange: ({ value }) => ({ fields: validateVolatilitySettings(value) }),
		},
	});
	return (
		<Paper
			component="form"
			onSubmit={(event) => {
				event.preventDefault();
				void form.handleSubmit();
			}}
			p={padding}
		>
			<Stack gap={spacing}>
				<SimpleGrid cols={columns} spacing={spacing}>
					<VolatilityFields
						fields={{ percentile: "percentile", period: "period" }}
						form={form}
						legend="Daily Volatility"
						percentilePresets={presets.percentilePresets}
						periodPresets={presets.days.periodPresets}
					/>
					<VolatilityFields
						fields={{ percentile: "hourlyPercentile", period: "hourlyPeriod" }}
						form={form}
						legend="Hourly Volatility"
						percentilePresets={presets.percentilePresets}
						periodPresets={presets.hours.periodPresets}
					/>
				</SimpleGrid>
				<form.AppForm>
					<form.SubmitButton disabled={disabled} loading={loading}>
						Apply settings
					</form.SubmitButton>
				</form.AppForm>
			</Stack>
		</Paper>
	);
}
