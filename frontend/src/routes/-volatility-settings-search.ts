import { defaultVolatilitySettings } from "@/features/analysis/criteria";
import { volatilitySettingsSchema } from "@/features/analysis/schemas";
import type {
	VolatilitySettings,
	VolatilitySettingsStateProps,
} from "@/features/analysis/volatility-settings-form/types";
import type { ResultsTableStateProps } from "@/features/market-scan/results-table/types";
import {
	type MarketScanSortSearch,
	parseMarketScanSortSearch,
	parseSymbolFilterSearch,
	resultsTableSearchProps,
	type SymbolFilterSearch,
} from "@/routes/-market-scan-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export interface VolatilitySettingsSearch {
	hourly_percentile: number;
	hourly_period: number;
	percentile: number;
	period: number;
}

export interface VolatilitySettingsSearchWithSort
	extends VolatilitySettingsSearch, MarketScanSortSearch, SymbolFilterSearch {}

export function parseVolatilitySettingsSearch(
	search: Record<string, unknown>,
): VolatilitySettingsSearchWithSort {
	const settings = volatilitySettingsFromUnknownSearch(search);

	return {
		...volatilitySettingsToSearch(settings ?? defaultVolatilitySettings),
		...parseMarketScanSortSearch(search),
		...parseSymbolFilterSearch(search),
	};
}

// Binds a page's volatility settings and results table state to its URL
// search.
export function volatilitySettingsSearchProps(
	search: VolatilitySettingsSearchWithSort,
): VolatilitySettingsStateProps & ResultsTableStateProps {
	return {
		initialSettings: volatilitySettingsFromSearch(search),
		onSettingsCommit: (settings) => {
			replaceUrlSearch({ ...volatilitySettingsToSearch(settings) });
		},
		...resultsTableSearchProps(search),
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
	return volatilitySettingsSchema.safeParse({
		hourlyPercentile: search.hourly_percentile,
		hourlyPeriod: search.hourly_period,
		percentile: search.percentile,
		period: search.period,
	}).data;
}
