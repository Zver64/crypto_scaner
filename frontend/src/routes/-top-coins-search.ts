import { applicationConfig } from "@/config";
import type { TopCoinsSettings } from "@/features/top-coins/settings-form/types";
import { settingsFromValidDraft } from "@/features/top-coins/settings-form/utils";
import type { MarketScanSortSearch } from "@/routes/-market-scan-search";
import { parseMarketScanSortSearch } from "@/routes/-market-scan-search";

export interface TopCoinsSettingsSearch {
	hourly_percentile: number;
	hourly_period: number;
	percentile: number;
	period: number;
}

export interface TopCoinsSearch
	extends TopCoinsSettingsSearch,
		MarketScanSortSearch {}

export function parseTopCoinsSearch(
	search: Record<string, unknown>,
): TopCoinsSearch {
	const settings = topCoinsSettingsFromUnknownSearch(search);

	return {
		...topCoinsSettingsToSearch(
			settings ?? applicationConfig.topMarketCap.defaultSettings,
		),
		...parseMarketScanSortSearch(search),
	};
}

export function topCoinsSettingsFromSearch(
	search: TopCoinsSettingsSearch,
): TopCoinsSettings {
	return {
		hourlyPercentile: search.hourly_percentile,
		hourlyPeriod: search.hourly_period,
		percentile: search.percentile,
		period: search.period,
	};
}

export function topCoinsSettingsToSearch(
	settings: TopCoinsSettings,
): TopCoinsSettingsSearch {
	return {
		hourly_percentile: settings.hourlyPercentile,
		hourly_period: settings.hourlyPeriod,
		percentile: settings.percentile,
		period: settings.period,
	};
}

function topCoinsSettingsFromUnknownSearch(
	search: Record<string, unknown>,
): TopCoinsSettings | undefined {
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
