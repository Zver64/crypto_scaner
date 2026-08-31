import { createFileRoute, useRouter } from "@tanstack/react-router";
import { InstrumentAnalysisPage } from "../features/instrument-analysis/instrument-analysis-page";
import {
	instrumentAnalysisCriteriaFromSearch,
	parseInstrumentAnalysisSearch,
} from "../features/instrument-analysis/search-params";

export const Route = createFileRoute("/instruments/$symbol")({
	component: InstrumentRoute,
	validateSearch: parseInstrumentAnalysisSearch,
});

function InstrumentRoute() {
	const { symbol } = Route.useParams();
	const search = Route.useSearch();
	const router = useRouter();
	const committedCriteria = instrumentAnalysisCriteriaFromSearch(search);

	return (
		<InstrumentAnalysisPage
			committedCriteria={committedCriteria}
			onBack={() => router.history.back()}
			symbol={symbol}
		/>
	);
}
