import { Stack } from "@mantine/core";

import { BacktestCoinSummary } from "@/features/strategy-backtest/backtest-coin-summary";
import type { BacktestRun } from "@/features/strategy-backtest/types";

interface BacktestMultiResultsProps {
	run: BacktestRun;
	gap: string;
	paperPadding: string;
}

export function BacktestMultiResults({
	run,
	gap,
	paperPadding,
}: BacktestMultiResultsProps) {
	return (
		<Stack gap={gap}>
			{run.items.map((item) => (
				<BacktestCoinSummary
					key={item.symbol}
					item={item}
					run={run}
					gap={gap}
					paperPadding={paperPadding}
				/>
			))}
		</Stack>
	);
}
