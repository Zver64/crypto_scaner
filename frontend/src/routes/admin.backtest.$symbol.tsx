import { createFileRoute } from "@tanstack/react-router";
import { StrategyBacktestScreen } from "@/features/strategy-backtest/strategy-backtest-screen";
import { parseStrategyBacktestSearch } from "@/routes/-strategy-backtest-search";

export const Route = createFileRoute("/admin/backtest/$symbol")({
	component: StrategyBacktestRoute,
	validateSearch: parseStrategyBacktestSearch,
});

function StrategyBacktestRoute() {
	const { symbol } = Route.useParams();
	const { strategy } = Route.useSearch();
	const navigate = Route.useNavigate();

	return (
		<StrategyBacktestScreen
			onStrategyChange={(next) =>
				void navigate({
					replace: true,
					search: (previous) => ({ ...previous, strategy: next }),
				})
			}
			strategy={strategy}
			symbol={symbol}
		/>
	);
}
