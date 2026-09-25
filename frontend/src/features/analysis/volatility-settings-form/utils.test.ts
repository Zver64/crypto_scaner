import { describe, expect, it } from "vitest";
import { applicationConfig } from "@/config";
import {
	settingsFromValidDraft,
	validateVolatilitySettings,
} from "@/features/analysis/volatility-settings-form/utils";

describe("Top Market Cap settings", () => {
	const defaultSettings = applicationConfig.topMarketCap.defaultSettings;

	it("rejects incomplete and invalid numeric drafts", () => {
		expect(
			settingsFromValidDraft({ ...defaultSettings, period: "30" }),
		).toBeUndefined();
		expect(
			settingsFromValidDraft({
				...defaultSettings,
				hourlyPercentile: 101,
			}),
		).toBeUndefined();
	});

	it("validates daily and hourly fields with analysis constraints", () => {
		expect(
			validateVolatilitySettings({
				...defaultSettings,
				hourlyPeriod: 0,
				percentile: 1.5,
			}),
		).toEqual({
			hourlyPeriod:
				"Analysis period must be a whole number between 1 and 87600 hours",
			percentile: "Range percentile must be a whole number",
		});
	});
});
