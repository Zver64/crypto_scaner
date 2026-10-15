import { createFileRoute } from "@tanstack/react-router";

import { FavoritesScreen } from "@/features/favorites/favorites-screen";
import {
	parseVolatilitySettingsSearch,
	volatilitySettingsSearchProps,
} from "@/routes/-volatility-settings-search";

export const Route = createFileRoute("/favorites")({
	component: FavoritesPage,
	validateSearch: parseVolatilitySettingsSearch,
});

function FavoritesPage() {
	return (
		<FavoritesScreen {...volatilitySettingsSearchProps(Route.useSearch())} />
	);
}
