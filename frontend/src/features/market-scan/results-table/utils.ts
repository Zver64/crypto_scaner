import { criterionKeys } from "@/api/analysis-identifiers";
import type { MarketScanItem, UnresolvedInstrumentCode } from "@/api/client";
import { volatilityEvaluation } from "@/features/market-scan/criteria";
import { marketCapEvaluation } from "@/utils/market-cap";

export interface MarketScanRow {
	symbol: string;
	dailyRangePercent: number | null;
	hourlyRangePercent: number | null;
	dailyCandleCount: number | null;
	hourlyCandleCount: number | null;
	marketCapUsd: number | null;
	priceHistory: readonly (number | null)[];
}

// Required evaluations are validated by the query. Presentation preserves every
// item, including unavailable optional metrics, without applying criteria again.
export function toMarketScanRows(
	items: readonly MarketScanItem[],
): MarketScanRow[] {
	return items.map((item) => {
		const daily = volatilityEvaluation(
			item.evaluations,
			criterionKeys.dailyVolatility,
		);
		const hourly = volatilityEvaluation(
			item.evaluations,
			criterionKeys.hourlyVolatility,
		);
		return {
			symbol: item.symbol,
			dailyRangePercent: daily?.rangePercent ?? null,
			hourlyRangePercent: hourly?.rangePercent ?? null,
			dailyCandleCount: daily?.candleCount ?? null,
			hourlyCandleCount: hourly?.candleCount ?? null,
			marketCapUsd: marketCapEvaluation(item.evaluations)?.marketCapUsd ?? null,
			priceHistory: item.price_history,
		};
	});
}

export function filterMarketScanRows(
	rows: readonly MarketScanRow[],
	filter: string,
): MarketScanRow[] {
	const normalizedFilter = filter.trim().toLocaleLowerCase("en-US");
	return rows.filter((row) =>
		row.symbol.toLocaleLowerCase("en-US").includes(normalizedFilter),
	);
}

const marketCapUnavailableReasons: Record<UnresolvedInstrumentCode, string> = {
	mapping_conflict:
		"Multiple market capitalization matches were found for this instrument.",
	mapping_not_found:
		"No market capitalization match was found for this instrument.",
	mapping_provider_unavailable:
		"Market capitalization mapping data is temporarily unavailable.",
	market_cap_missing:
		"Market capitalization data is unavailable for this instrument.",
};

export function marketCapUnavailableReason(
	code: UnresolvedInstrumentCode,
): string {
	return marketCapUnavailableReasons[code];
}
