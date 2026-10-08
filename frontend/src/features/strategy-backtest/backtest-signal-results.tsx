import { Stack, Text } from "@mantine/core";
import type { BacktestSignal, StrategyBacktest } from "@/api/generated/models";
import { EmptyState } from "@/components/empty-state";
import { BacktestSignalTable } from "@/features/strategy-backtest/backtest-signal-table";
import { BacktestSignalWindows } from "@/features/strategy-backtest/backtest-signal-windows";
import { signalHitHints } from "@/features/strategy-backtest/constants";
import { noCandlesMessage } from "@/features/strategy-backtest/utils";

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
			<BacktestSignalWindows windows={signal.windows} />
			<Text c="dimmed" size="sm">
				Median moves from the close of the signal candle over the next candles:
				the rise to the highest high, the fall to the lowest low, and the range
				between them. Hits are the share of moves with{" "}
				{signalHitHints[signal.direction]}; All candles shows the same after
				every evaluated candle.
			</Text>
			<BacktestSignalTable
				direction={signal.direction}
				occurrences={signal.occurrences}
				windows={signal.windows}
			/>
		</Stack>
	);
}
