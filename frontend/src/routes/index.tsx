import { createFileRoute } from "@tanstack/react-router";

import { MarketScanScreen } from "@/features/market-scan/market-scan-screen";
import {
	parseMarketScanSearch,
	resultsTableSearchProps,
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
