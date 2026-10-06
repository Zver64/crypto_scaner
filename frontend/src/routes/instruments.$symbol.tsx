import {
	createFileRoute,
	useCanGoBack,
	useNavigate,
	useRouter,
} from "@tanstack/react-router";
import { InstrumentAnalysisScreen } from "@/features/instrument-analysis/instrument-analysis-screen";
import { volatilityCriterionSelections } from "@/features/market-scan/pipeline";
import {
	parseRequiredScanCriteriaSearch,
	requiredScanCriteriaFromSearch,
	scanCriteriaToSearch,
} from "@/routes/-scan-criteria-search";

export const Route = createFileRoute("/instruments/$symbol")({
	component: InstrumentRoute,
	validateSearch: parseRequiredScanCriteriaSearch,
	beforeLoad: ({ params }) => ({ pageTitle: params.symbol.toUpperCase() }),
});

function InstrumentRoute() {
	const { symbol } = Route.useParams();
	const search = Route.useSearch();
	const router = useRouter();
	const navigate = useNavigate();
	const canGoBack = useCanGoBack();
	const criteria = requiredScanCriteriaFromSearch(search);

	return (
		<InstrumentAnalysisScreen
			criterionSelections={volatilityCriterionSelections(criteria)}
			// A coin opened directly (a link or a reload) has no page to go back
			// to, so it returns to the Market Scan with its criteria.
			onBack={() => {
				if (canGoBack) {
					router.history.back();
				} else {
					void navigate({ to: "/", search: scanCriteriaToSearch(criteria) });
				}
			}}
			symbol={symbol}
		/>
	);
}
