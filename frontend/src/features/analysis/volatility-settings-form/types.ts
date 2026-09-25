export interface VolatilitySettings {
	hourlyPercentile: number;
	hourlyPeriod: number;
	percentile: number;
	period: number;
}

export interface UseVolatilitySettingsFormOptions {
	disabled: boolean;
	initialSettings: VolatilitySettings;
	onCommit(settings: VolatilitySettings): void;
}

export type VolatilitySettingsDraft = {
	[Field in keyof VolatilitySettings]: number | string;
};

export type VolatilitySettingsValidationErrors = Partial<
	Record<keyof VolatilitySettings, string>
>;
