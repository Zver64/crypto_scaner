export interface VolatilitySettings {
	hourlyPercentile: number;
	hourlyPeriod: number;
	percentile: number;
	period: number;
}

export type VolatilitySettingsDraft = {
	[Field in keyof VolatilitySettings]: number | string;
};

export type VolatilitySettingsValidationErrors = Partial<
	Record<keyof VolatilitySettings, string>
>;
