import { createFileRoute, redirect } from "@tanstack/react-router";

import { defaultVolatilitySettings } from "@/features/analysis/criteria";
import { MarketScanScreen } from "@/features/market-scan/market-scan-screen";
import { parseLaunchSearch } from "@/routes/-launch-search";
import {
	parseMarketScanSearch,
	resultsTableSearchProps,
} from "@/routes/-market-scan-search";
import {
	scanCriteriaFromSearch,
	scanCriteriaToSearch,
} from "@/routes/-scan-criteria-search";
import { volatilitySettingsToSearch } from "@/routes/-volatility-settings-search";
import { replaceUrlSearch } from "@/utils/replace-url-search";

export const Route = createFileRoute("/")({
	component: Home,
	validateSearch: (search) => ({
		...parseMarketScanSearch(search),
		...parseLaunchSearch(search),
	}),
	// A coin link from a bot message opens the Mini App here; it shows the
	// coin with the default volatility settings, replacing the launch URL, so a reload
	// stays on the page shown then.
	beforeLoad: ({ search }) => {
		if (search.tgWebAppStartParam) {
			throw redirect({
				to: "/instruments/$symbol",
				params: { symbol: search.tgWebAppStartParam },
				search: volatilitySettingsToSearch(defaultVolatilitySettings),
				replace: true,
			});
		}
	},
});

function Home() {
	const search = Route.useSearch();

	return (
		<MarketScanScreen
			initialCriteria={scanCriteriaFromSearch(search)}
			onCriteriaCommit={(criteria) => {
				replaceUrlSearch(scanCriteriaToSearch(criteria));
			}}
			{...resultsTableSearchProps(search)}
		/>
	);
}
