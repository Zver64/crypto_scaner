import { createFileRoute, useRouter } from "@tanstack/react-router";
import { InstrumentAnalysisScreen } from "@/features/instrument-analysis/instrument-analysis-screen";
import { criterionSelections } from "@/features/market-scan/pipeline";
import {
	parseRequiredScanCriteriaSearch,
	requiredScanCriteriaFromSearch,
} from "@/routes/-scan-criteria-search";

export const Route = createFileRoute("/instruments/$symbol")({
	component: InstrumentRoute,
	validateSearch: parseRequiredScanCriteriaSearch,
});

function InstrumentRoute() {
	const { symbol } = Route.useParams();
	const search = Route.useSearch();
	const router = useRouter();
	const criteria = requiredScanCriteriaFromSearch(search);

	return (
		<InstrumentAnalysisScreen
			criterionSelections={criterionSelections(criteria)}
			onBack={() => router.history.back()}
			symbol={symbol}
		/>
	);
}
