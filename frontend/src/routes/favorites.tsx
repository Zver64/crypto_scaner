import { createFileRoute } from "@tanstack/react-router";
import { FavoritesScreen } from "@/features/favorites/favorites-screen";
import { marketScanSortFromSearch } from "@/routes/-market-scan-search";
import {
	parseVolatilitySettingsSearch,
	volatilitySettingsFromSearch,
	volatilitySettingsToSearch,
} from "@/routes/-volatility-settings-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/favorites")({
	component: FavoritesPage,
	validateSearch: parseVolatilitySettingsSearch,
});

function FavoritesPage() {
	const search = Route.useSearch();
	return (
		<FavoritesScreen
			initialSettings={volatilitySettingsFromSearch(search)}
			onSettingsCommit={(settings) => {
				replaceUrlSearch({ ...volatilitySettingsToSearch(settings) });
			}}
			onSortChange={(sort) => {
				replaceUrlSearch({
					sort_column: sort.column,
					sort_direction: sort.direction,
				});
			}}
			sort={marketScanSortFromSearch(search)}
		/>
	);
}
