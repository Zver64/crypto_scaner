import type { MarketScanSort } from "@/features/market-scan/sort";

// Results table state a page keeps in its URL.
export interface ResultsTableStateProps {
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}
