import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { StrategyBacktestCoins } from "@/features/strategy-backtest/strategy-backtest-coins";
import { marketScanSortFromSearch } from "@/routes/-market-scan-search";
import { parseStrategyBacktestSearch } from "@/routes/-strategy-backtest-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/admin/backtest/")({
	component: StrategyBacktestCoinsPage,
	validateSearch: parseStrategyBacktestSearch,
});

function StrategyBacktestCoinsPage() {
	const search = Route.useSearch();
	const navigate = useNavigate();
	return (
		<StrategyBacktestCoins
			// Replacing keeps a single settings history entry; the coin page
			// carries the list state to restore it.
			onCoinClick={(symbol) =>
				void navigate({
					params: { symbol },
					replace: true,
					search,
					to: "/admin/backtest/$symbol",
				})
			}
			onSortChange={(sort) => {
				replaceUrlSearch({
					sort_column: sort.column,
					sort_direction: sort.direction,
				});
			}}
			onSymbolFilterChange={(symbolFilter) => {
				replaceUrlSearch({ symbol_filter: symbolFilter || undefined });
			}}
			sort={marketScanSortFromSearch(search)}
			symbolFilter={search.symbol_filter ?? ""}
		/>
	);
}
