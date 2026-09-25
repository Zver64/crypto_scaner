import { describe, expect, it } from "vitest";
import { applicationConfig } from "@/config";
import { marketScanSortFromSearch } from "@/routes/-market-scan-search";
import {
	parseVolatilitySettingsSearch,
	volatilitySettingsFromSearch,
	volatilitySettingsToSearch,
} from "@/routes/-volatility-settings-search";

const defaultSearch = {
	hourly_percentile: 80,
	hourly_period: 60,
	percentile: 80,
	period: 30,
};

const customSearch = {
	hourly_percentile: 95,
	hourly_period: 72,
	percentile: 90,
	period: 60,
};

describe("Top Market Cap URL state", () => {
	it("uses all default settings when search is absent", () => {
		expect(parseVolatilitySettingsSearch({})).toEqual(defaultSearch);
		expect(
			volatilitySettingsFromSearch(parseVolatilitySettingsSearch({})),
		).toEqual(applicationConfig.topMarketCap.defaultSettings);
	});

	it("accepts a complete valid settings group", () => {
		const parsed = parseVolatilitySettingsSearch(customSearch);

		expect(parsed).toEqual(customSearch);
		expect(volatilitySettingsFromSearch(parsed)).toEqual({
			hourlyPercentile: 95,
			hourlyPeriod: 72,
			percentile: 90,
			period: 60,
		});
	});

	it.each([
		[
			"missing",
			{
				hourly_percentile: 95,
				percentile: 90,
				period: 60,
			},
		],
		["partial", { period: 60, percentile: 90 }],
		["nonnumeric", { ...customSearch, hourly_period: "72" }],
		["invalid", { ...customSearch, percentile: 101 }],
	] as const)("atomically replaces a %s settings group with defaults", (_, search) => {
		expect(parseVolatilitySettingsSearch(search)).toEqual(defaultSearch);
	});

	it("keeps valid sort when settings are invalid", () => {
		const parsed = parseVolatilitySettingsSearch({
			...customSearch,
			period: 0,
			sort_column: "hourlyRangePercent",
			sort_direction: "asc",
		});

		expect(parsed).toEqual({
			...defaultSearch,
			sort_column: "hourlyRangePercent",
			sort_direction: "asc",
		});
		expect(marketScanSortFromSearch(parsed)).toEqual({
			column: "hourlyRangePercent",
			direction: "asc",
		});
	});

	it("serializes all four committed settings without overwriting sort", () => {
		const currentSearch = parseVolatilitySettingsSearch({
			...defaultSearch,
			sort_column: "marketCapUsd",
			sort_direction: "desc",
		});
		const settingsUpdate = volatilitySettingsToSearch({
			hourlyPercentile: 95,
			hourlyPeriod: 72,
			percentile: 90,
			period: 60,
		});

		expect(settingsUpdate).toEqual(customSearch);
		expect({ ...currentSearch, ...settingsUpdate }).toEqual({
			...customSearch,
			sort_column: "marketCapUsd",
			sort_direction: "desc",
		});
	});

	it("preserves settings when sort is updated key-wise", () => {
		const currentSearch = parseVolatilitySettingsSearch(customSearch);
		const sortUpdate = {
			sort_column: "dailyRangePercent" as const,
			sort_direction: "asc" as const,
		};

		expect({ ...currentSearch, ...sortUpdate }).toEqual({
			...customSearch,
			...sortUpdate,
		});
	});
});
