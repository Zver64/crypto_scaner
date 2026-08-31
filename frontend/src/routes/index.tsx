import { createFileRoute } from "@tanstack/react-router";
import { instrumentAnalysisCriteriaToSearch } from "../features/instrument-analysis/search-params";
import { MarketScanPage } from "../features/market-scan/market-scan-page";
import {
	marketScanCriteriaFromSearch,
	marketScanCriteriaToSearch,
	parseMarketScanSearch,
} from "../features/market-scan/search-params";

export const Route = createFileRoute("/")({
	component: Home,
	validateSearch: parseMarketScanSearch,
});

function Home() {
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	const committedCriteria = marketScanCriteriaFromSearch(search);

	return (
		<MarketScanPage
			committedCriteria={committedCriteria}
			onCommit={async (criteria) => {
				await navigate({ search: marketScanCriteriaToSearch(criteria) });
			}}
			onSelectInstrument={async (symbol, criteria) => {
				await navigate({
					params: { symbol },
					search: instrumentAnalysisCriteriaToSearch({
						...criteria,
						unit: "days",
					}),
					to: "/instruments/$symbol",
				});
			}}
		/>
	);
}
