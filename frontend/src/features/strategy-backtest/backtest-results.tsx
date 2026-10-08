import { SimpleGrid, Stack } from "@mantine/core";
import { memo } from "react";
import type { StrategyBacktest } from "@/api/generated/models";
import { EmptyState } from "@/components/empty-state";
import { BacktestEquityChart } from "@/features/strategy-backtest/backtest-equity-chart";
import { BacktestMetricCards } from "@/features/strategy-backtest/backtest-metric-cards";
import { BacktestSignalResults } from "@/features/strategy-backtest/backtest-signal-results";
import { BacktestSummary } from "@/features/strategy-backtest/backtest-summary";
import { BacktestTradesTable } from "@/features/strategy-backtest/backtest-trades-table";
import { noCandlesMessage } from "@/features/strategy-backtest/utils";

interface BacktestResultsProps {
	backtest: StrategyBacktest;
	// Whether a period limited the backtest.
	period: boolean;
	gap: string;
	paperPadding: string;
}

// The metric cards, then the averages and the equity side by side from
// tablets on, then the trades, as a signal shows its
// results above its signals. Without trades, one empty state replaces them all. A
// signal shows its signals and the moves after them instead.
export const BacktestResults = memo(function BacktestResults({
	backtest,
	gap,
	paperPadding,
	period,
}: BacktestResultsProps) {
	if (backtest.signal) {
		return (
			<BacktestSignalResults
				backtest={backtest}
				gap={gap}
				period={period}
				signal={backtest.signal}
			/>
		);
	}
	if (backtest.trades.length === 0) {
		return (
			<EmptyState
				description={
					backtest.from === null
						? noCandlesMessage(period)
						: "The strategy did not buy on this coin in the stored history."
				}
				title="No trades"
			/>
		);
	}
	return (
		<Stack gap={gap}>
			<BacktestMetricCards
				backtest={backtest}
				gap={gap}
				paperPadding={paperPadding}
			/>
			<SimpleGrid cols={{ base: 1, md: 2 }} spacing={gap}>
				<BacktestSummary backtest={backtest} />
				<BacktestEquityChart backtest={backtest} paperPadding={paperPadding} />
			</SimpleGrid>
			<BacktestTradesTable backtest={backtest} />
		</Stack>
	);
});
