import { createFileRoute } from "@tanstack/react-router";
import { MarketScanScreen } from "@/features/market-scan/market-scan-screen";
import {
	marketScanSortFromSearch,
	parseMarketScanSearch,
} from "@/routes/-market-scan-search";
import {
	scanCriteriaFromSearch,
	scanCriteriaToSearch,
} from "@/routes/-scan-criteria-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/")({
	component: Home,
	validateSearch: parseMarketScanSearch,
});

function Home() {
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	const initialCriteria = scanCriteriaFromSearch(search);
	const sort = marketScanSortFromSearch(search);

	return (
		<MarketScanScreen
			initialCriteria={initialCriteria}
			onCriteriaCommit={(criteria) => {
				replaceUrlSearch(scanCriteriaToSearch(criteria));
			}}
			onSortChange={(nextSort) => {
				replaceUrlSearch({
					sort_column: nextSort.column,
					sort_direction: nextSort.direction,
				});
			}}
			onSymbolFilterChange={(symbolFilter) => {
				replaceUrlSearch({ symbol_filter: symbolFilter || undefined });
			}}
			onSelectInstrument={async (symbol, criteria) => {
				await navigate({
					params: { symbol },
					search: scanCriteriaToSearch(criteria),
					to: "/instruments/$symbol",
				});
			}}
			sort={sort}
			symbolFilter={search.symbol_filter ?? ""}
		/>
	);
}
