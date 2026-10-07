import { SimpleGrid, Stack, Text } from "@mantine/core";
import { memo } from "react";
import type { StrategyBacktest } from "@/api/generated/models";
import { EmptyState } from "@/components/empty-state";
import { BacktestEquityChart } from "@/features/strategy-backtest/backtest-equity-chart";
import { BacktestMetricCards } from "@/features/strategy-backtest/backtest-metric-cards";
import { BacktestSummary } from "@/features/strategy-backtest/backtest-summary";
import { BacktestTradesTable } from "@/features/strategy-backtest/backtest-trades-table";
import { formatDateTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";

interface BacktestResultsProps {
	backtest: StrategyBacktest;
	gap: string;
	paperPadding: string;
}

// The trades, the evaluated period and the metric cards, then the averages and the equity side by side
// from tablets on. Without closed trades, one empty state replaces them all.
export const BacktestResults = memo(function BacktestResults({
	backtest,
	gap,
	paperPadding,
}: BacktestResultsProps) {
	if (backtest.trades.length === 0) {
		const unfinished = backtest.unfinished_trades;
		return (
			<EmptyState
				description={
					backtest.from === null
						? "No stored candles of this interval yet; synchronization fills them first."
						: unfinished > 0
							? `Every trade is still unfinished (${formatNumber(unfinished)}): the stored history ends or has a gap before its exit.`
							: "The strategy did not alert on this coin in the stored history."
				}
				title="No trades"
			/>
		);
	}
	return (
		<Stack gap={gap}>
			<BacktestTradesTable backtest={backtest} />
			{backtest.from && backtest.to ? (
				<Text c="dimmed" size="sm">
					Period: {formatDateTime(backtest.from)} →{" "}
					{formatDateTime(backtest.to)} ({backtest.interval})
				</Text>
			) : null}
			<BacktestMetricCards
				backtest={backtest}
				gap={gap}
				paperPadding={paperPadding}
			/>
			<SimpleGrid cols={{ base: 1, md: 2 }} spacing={gap}>
				<BacktestSummary backtest={backtest} />
				<BacktestEquityChart backtest={backtest} paperPadding={paperPadding} />
			</SimpleGrid>
		</Stack>
	);
});
