import { createFileRoute } from "@tanstack/react-router";
import { TopCoinsScreen } from "@/features/top-coins/top-coins-screen";
import { marketScanSortFromSearch } from "@/routes/-market-scan-search";
import {
	parseVolatilitySettingsSearch,
	volatilitySettingsFromSearch,
	volatilitySettingsToSearch,
} from "@/routes/-volatility-settings-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/top-coins")({
	component: TopCoinsPage,
	validateSearch: parseVolatilitySettingsSearch,
});

function TopCoinsPage() {
	const search = Route.useSearch();
	const sort = marketScanSortFromSearch(search);

	return (
		<TopCoinsScreen
			initialSettings={volatilitySettingsFromSearch(search)}
			onSettingsCommit={(settings) => {
				replaceUrlSearch({ ...volatilitySettingsToSearch(settings) });
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
