import { createFileRoute } from "@tanstack/react-router";
import { StrategyBacktestScreen } from "@/features/strategy-backtest/strategy-backtest-screen";
import { parseStrategyBacktestSearch } from "@/routes/-strategy-backtest-search";

export const Route = createFileRoute("/admin/backtest/")({
	component: StrategyBacktestPage,
	validateSearch: parseStrategyBacktestSearch,
});

function StrategyBacktestPage() {
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	return (
		<StrategyBacktestScreen
			allCoins={search.all_coins === true}
			hold={search.hold}
			onCoinChange={(symbol, allCoins) =>
				void navigate({
					replace: true,
					search: (previous) => ({
						...previous,
						all_coins: allCoins || undefined,
						symbol,
					}),
				})
			}
			onHoldChange={(hold) =>
				void navigate({
					replace: true,
					search: (previous) => ({ ...previous, hold }),
				})
			}
			onStrategyChange={(strategy) =>
				void navigate({
					replace: true,
					search: (previous) => ({ ...previous, strategy }),
				})
			}
			strategy={search.strategy}
			symbol={search.symbol}
		/>
	);
}
