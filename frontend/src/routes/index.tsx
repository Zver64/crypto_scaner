import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { MarketScanScreen } from "@/features/market-scan/market-scan-screen";
import { defaultMarketScanSort } from "@/features/market-scan/sort";
import {
	parseOptionalScanCriteriaSearch,
	scanCriteriaFromSearch,
	scanCriteriaToSearch,
} from "@/routes/-scan-criteria-search";

export const Route = createFileRoute("/")({
	component: Home,
	validateSearch: parseOptionalScanCriteriaSearch,
});

function Home() {
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	const committedCriteria = scanCriteriaFromSearch(search);
	const [sort, setSort] = useState(defaultMarketScanSort);

	return (
		<MarketScanScreen
			committedCriteria={committedCriteria}
			onCommit={async (criteria) => {
				await navigate({
					search: scanCriteriaToSearch(criteria),
				});
			}}
			onSortChange={async (nextSort) => {
				setSort(nextSort);
			}}
			onSelectInstrument={async (symbol, criteria) => {
				await navigate({
					params: { symbol },
					search: scanCriteriaToSearch(criteria),
					to: "/instruments/$symbol",
				});
			}}
			sort={sort}
		/>
	);
}
