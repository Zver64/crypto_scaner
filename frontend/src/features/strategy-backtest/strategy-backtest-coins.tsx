import { Stack, Text } from "@mantine/core";
import { applicationConfig } from "@/config";
import { FavoritesTable } from "@/features/favorites/favorites-table";
import { useFavoritesAnalysis } from "@/features/favorites/use-favorites-analysis";
import type { MarketScanSort } from "@/features/market-scan/sort";

interface StrategyBacktestCoinsProps {
	onCoinClick(symbol: string): void;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}

// The administrator's favorites, the coins strategies run on; a coin opens its
// backtest.
export function StrategyBacktestCoins({
	onCoinClick,
	...table
}: StrategyBacktestCoinsProps) {
	// The list has no settings form; the table uses the default settings.
	const analysis = useFavoritesAnalysis(
		applicationConfig.topMarketCap.defaultSettings,
	);
	return (
		<Stack gap="md">
			<Text c="dimmed" size="sm">
				Choose a coin to see where a strategy would have alerted.
			</Text>
			<FavoritesTable analysis={analysis} onRowClick={onCoinClick} {...table} />
		</Stack>
	);
}
