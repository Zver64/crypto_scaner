import { createFileRoute, useRouter } from "@tanstack/react-router";
import { InstrumentAnalysisPage } from "../features/instrument-analysis/instrument-analysis-page";
import {
	instrumentAnalysisCriteriaFromSearch,
	instrumentAnalysisCriteriaToSearch,
	parseInstrumentAnalysisSearch,
} from "../features/instrument-analysis/search-params";

export const Route = createFileRoute("/instruments/$symbol")({
	component: InstrumentRoute,
	validateSearch: parseInstrumentAnalysisSearch,
});

function InstrumentRoute() {
	const { symbol } = Route.useParams();
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	const router = useRouter();
	const committedCriteria = instrumentAnalysisCriteriaFromSearch(search);

	return (
		<InstrumentAnalysisPage
			committedCriteria={committedCriteria}
			onBack={() => router.history.back()}
			onCommit={async (criteria) => {
				await navigate({
					replace: true,
					search: instrumentAnalysisCriteriaToSearch(criteria),
				});
			}}
			symbol={symbol}
		/>
	);
}
