import { applicationConfig } from "@/config";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { settingsFromValidDraft } from "@/features/analysis/volatility-settings-form/utils";
import type { MarketScanSortSearch } from "@/routes/-market-scan-search";
import { parseMarketScanSortSearch } from "@/routes/-market-scan-search";

export interface VolatilitySettingsSearch {
	hourly_percentile: number;
	hourly_period: number;
	percentile: number;
	period: number;
}

export interface VolatilitySettingsSearchWithSort
	extends VolatilitySettingsSearch,
		MarketScanSortSearch {}

export function parseVolatilitySettingsSearch(
	search: Record<string, unknown>,
): VolatilitySettingsSearchWithSort {
	const settings = volatilitySettingsFromUnknownSearch(search);

	return {
		...volatilitySettingsToSearch(
			settings ?? applicationConfig.topMarketCap.defaultSettings,
		),
		...parseMarketScanSortSearch(search),
	};
}

export function volatilitySettingsFromSearch(
	search: VolatilitySettingsSearch,
): VolatilitySettings {
	return {
		hourlyPercentile: search.hourly_percentile,
		hourlyPeriod: search.hourly_period,
		percentile: search.percentile,
		period: search.period,
	};
}

export function volatilitySettingsToSearch(
	settings: VolatilitySettings,
): VolatilitySettingsSearch {
	return {
		hourly_percentile: settings.hourlyPercentile,
		hourly_period: settings.hourlyPeriod,
		percentile: settings.percentile,
		period: settings.period,
	};
}

function volatilitySettingsFromUnknownSearch(
	search: Record<string, unknown>,
): VolatilitySettings | undefined {
	const { hourly_percentile, hourly_period, percentile, period } = search;
	if (
		typeof period !== "number" ||
		typeof percentile !== "number" ||
		typeof hourly_period !== "number" ||
		typeof hourly_percentile !== "number"
	) {
		return undefined;
	}

	return settingsFromValidDraft({
		hourlyPercentile: hourly_percentile,
		hourlyPeriod: hourly_period,
		percentile,
		period,
	});
}
