import { Stack, Text } from "@mantine/core";
import type { BacktestSignal, StrategyBacktest } from "@/api/generated/models";
import { EmptyState } from "@/components/empty-state";
import { BacktestSignalSummary } from "@/features/strategy-backtest/backtest-signal-summary";
import { BacktestSignalTable } from "@/features/strategy-backtest/backtest-signal-table";
import {
	noCandlesMessage,
	signalExplanation,
} from "@/features/strategy-backtest/utils";

interface BacktestSignalResultsProps {
	backtest: StrategyBacktest;
	gap: string;
	// Whether a period limited the backtest.
	period: boolean;
	signal: BacktestSignal;
}

// How the price moved after the signals compared with every candle, then the
// signals themselves. Without signals, one empty state replaces them.
export function BacktestSignalResults({
	backtest,
	gap,
	period,
	signal,
}: BacktestSignalResultsProps) {
	if (signal.occurrences.length === 0) {
		return (
			<EmptyState
				description={
					backtest.from === null
						? noCandlesMessage(period)
						: "The entry did not turn true on this coin in the stored history."
				}
				title="No signals"
			/>
		);
	}
	return (
		<Stack gap={gap}>
			<Text c="dimmed" size="sm">
				{signalExplanation(
					backtest.direction,
					signal.window,
					signal.target_ratio,
				)}
			</Text>
			<BacktestSignalSummary signal={signal} />
			<BacktestSignalTable
				direction={backtest.direction}
				occurrences={signal.occurrences}
			/>
		</Stack>
	);
}
