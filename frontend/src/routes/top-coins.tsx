import { createFileRoute } from "@tanstack/react-router";
import { TopCoinsScreen } from "@/features/top-coins/top-coins-screen";
import { marketScanSortFromSearch } from "@/routes/-market-scan-search";
import {
	parseTopCoinsSearch,
	topCoinsSettingsFromSearch,
	topCoinsSettingsToSearch,
} from "@/routes/-top-coins-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/top-coins")({
	component: TopCoinsPage,
	validateSearch: parseTopCoinsSearch,
});

function TopCoinsPage() {
	const search = Route.useSearch();
	const sort = marketScanSortFromSearch(search);

	return (
		<TopCoinsScreen
			initialSettings={topCoinsSettingsFromSearch(search)}
			onSettingsCommit={(settings) => {
				replaceUrlSearch({ ...topCoinsSettingsToSearch(settings) });
			}}
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
