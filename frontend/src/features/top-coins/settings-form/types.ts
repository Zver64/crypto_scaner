export interface TopCoinsSettings {
	hourlyPercentile: number;
	hourlyPeriod: number;
	percentile: number;
	period: number;
}

export interface TopCoinsSettingsFormProps {
	disabled: boolean;
	initialSettings: TopCoinsSettings;
	onCommit(settings: TopCoinsSettings): void;
}

export type TopCoinsSettingsDraft = {
	[Field in keyof TopCoinsSettings]: number | string;
};

export type TopCoinsSettingsValidationErrors = Partial<
	Record<keyof TopCoinsSettings, string>
>;
