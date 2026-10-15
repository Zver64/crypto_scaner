import { createFileRoute } from "@tanstack/react-router";

import { TopCoinsScreen } from "@/features/top-coins/top-coins-screen";
import {
	parseVolatilitySettingsSearch,
	volatilitySettingsSearchProps,
} from "@/routes/-volatility-settings-search";

export const Route = createFileRoute("/top-coins")({
	component: TopCoinsPage,
	validateSearch: parseVolatilitySettingsSearch,
});

function TopCoinsPage() {
	return (
		<TopCoinsScreen
			page="top-coins"
			{...volatilitySettingsSearchProps(Route.useSearch())}
		/>
	);
}
