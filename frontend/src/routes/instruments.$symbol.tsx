import {
	createFileRoute,
	useCanGoBack,
	useNavigate,
	useRouter,
} from "@tanstack/react-router";

import { InstrumentAnalysisScreen } from "@/features/instrument-analysis/instrument-analysis-screen";
import {
	defaultMarketScanCriteria,
	volatilityCriterionSelections,
} from "@/features/market-scan/pipeline";
import { scanCriteriaToSearch } from "@/routes/-scan-criteria-search";
import {
	parseVolatilitySettingsOnlySearch,
	volatilitySettingsFromSearch,
} from "@/routes/-volatility-settings-search";

export const Route = createFileRoute("/instruments/$symbol")({
	component: InstrumentRoute,
	validateSearch: parseVolatilitySettingsOnlySearch,
	beforeLoad: ({ params }) => ({ pageTitle: params.symbol.toUpperCase() }),
});

function InstrumentRoute() {
	const { symbol } = Route.useParams();
	const settings = volatilitySettingsFromSearch(Route.useSearch());
	const router = useRouter();
	const navigate = useNavigate();
	const canGoBack = useCanGoBack();

	return (
		<InstrumentAnalysisScreen
			criterionSelections={volatilityCriterionSelections(settings)}
			// A coin opened directly (a link or a reload) has no page to go back
			// to, so it returns to the Market Scan with its volatility settings
			// and the default filters.
			onBack={() => {
				if (canGoBack) {
					router.history.back();
				} else {
					void navigate({
						to: "/",
						search: scanCriteriaToSearch({
							...defaultMarketScanCriteria,
							...settings,
						}),
					});
				}
			}}
			symbol={symbol}
		/>
	);
}
