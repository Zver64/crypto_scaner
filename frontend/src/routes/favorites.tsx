import { createFileRoute } from "@tanstack/react-router";
import { FavoritesScreen } from "@/features/favorites/favorites-screen";
import {
	marketScanSortFromSearch,
	parseMarketScanSortSearch,
} from "@/routes/-market-scan-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/favorites")({
	component: FavoritesPage,
	validateSearch: parseMarketScanSortSearch,
});

function FavoritesPage() {
	const search = Route.useSearch();
	return (
		<FavoritesScreen
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
