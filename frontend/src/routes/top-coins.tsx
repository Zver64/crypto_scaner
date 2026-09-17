import { createFileRoute } from "@tanstack/react-router";
import { TopCoinsScreen } from "@/features/top-coins/top-coins-screen";
import {
	marketScanSortFromSearch,
	parseMarketScanSortSearch,
} from "@/routes/-market-scan-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/top-coins")({
	component: TopCoinsPage,
	validateSearch: parseMarketScanSortSearch,
});

function TopCoinsPage() {
	const sort = marketScanSortFromSearch(Route.useSearch());

	return (
		<TopCoinsScreen
			onSortChange={(nextSort) => {
				replaceUrlSearch({
					sort_column: nextSort.column,
					sort_direction: nextSort.direction,
				});
			}}
			sort={sort}
		/>
	);
}
