import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { instrumentAnalysisCriteriaToSearch } from "../features/instrument-analysis/search-params";
import { MarketScanPage } from "../features/market-scan/page";
import {
	marketScanCriteriaFromSearch,
	marketScanCriteriaToSearch,
	parseMarketScanSearch,
} from "../features/market-scan/search-params";
import { defaultMarketScanSort } from "../features/market-scan/sort";

export const Route = createFileRoute("/")({
	component: Home,
	validateSearch: parseMarketScanSearch,
});

function Home() {
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	const committedCriteria = marketScanCriteriaFromSearch(search);
	const [sort, setSort] = useState(defaultMarketScanSort);

	return (
		<MarketScanPage
			committedCriteria={committedCriteria}
			onCommit={async (criteria) => {
				await navigate({
					search: marketScanCriteriaToSearch(criteria),
				});
			}}
			onSortChange={async (nextSort) => {
				setSort(nextSort);
			}}
			onSelectInstrument={async (symbol, criteria) => {
				await navigate({
					params: { symbol },
					search: instrumentAnalysisCriteriaToSearch(criteria),
					to: "/instruments/$symbol",
				});
			}}
			sort={sort}
		/>
	);
}
