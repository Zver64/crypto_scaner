import { createFileRoute } from "@tanstack/react-router";

import { TopCoinsScreen } from "@/features/top-coins/top-coins-screen";
import {
	parseVolatilitySettingsSearch,
	volatilitySettingsSearchProps,
} from "@/routes/-volatility-settings-search";

export const Route = createFileRoute("/coin-m")({
	component: CoinMPage,
	validateSearch: parseVolatilitySettingsSearch,
});

function CoinMPage() {
	return (
		<TopCoinsScreen
			page="coin-m"
			{...volatilitySettingsSearchProps(Route.useSearch())}
		/>
	);
}
