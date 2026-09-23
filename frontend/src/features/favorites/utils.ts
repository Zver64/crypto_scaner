import type { Favorite, MarketAnalysisItem } from "@/api/generated/models";
import {
	type MarketScanRow,
	toMarketScanRows,
} from "@/features/market-scan/results-table/utils";

export function mergeFavoriteRows(
	favorites: readonly Favorite[],
	analysisItems: readonly MarketAnalysisItem[],
): MarketScanRow[] {
	const analyzed = new Map(
		toMarketScanRows(analysisItems).map((row) => [row.symbol, row]),
	);
	return favorites.map(
		(favorite) =>
			analyzed.get(favorite.symbol) ?? {
				symbol: favorite.symbol,
				dailyRangePercent: null,
				hourlyRangePercent: null,
				marketCapUsd: null,
				priceHistory: [],
				sevenDayChangePercent: null,
			},
	);
}

export function favoriteAlertCounts(favorites: readonly Favorite[]) {
	return new Map(favorites.map((item) => [item.symbol, item.alert_count]));
}
