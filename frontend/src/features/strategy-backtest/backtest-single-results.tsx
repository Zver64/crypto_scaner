import { Stack, Text } from "@mantine/core";

import { EmptyState } from "@/components/empty-state";
import { BacktestChart } from "@/features/strategy-backtest/backtest-chart";
import { BacktestResults } from "@/features/strategy-backtest/backtest-results";
import type { BacktestRun } from "@/features/strategy-backtest/types";

interface BacktestSingleResultsProps {
	run: BacktestRun;
	gap: string;
	paperPadding: string;
	wide: boolean;
	enabled: boolean;
}

export function BacktestSingleResults({
	run,
	gap,
	paperPadding,
	wide,
	enabled,
}: BacktestSingleResultsProps) {
	const item = run.items[0];
	const result = item.result;
	if (!result) {
		const description =
			item.error ??
			(item.status === "cancelled" ? "Cancelled" : "Running the backtest…");
		return wide ? (
			<EmptyState
				description={description}
				failed={item.status === "failed"}
				fillHeight
				title={
					item.status === "failed"
						? "The backtest could not be run."
						: "Backtest"
				}
			/>
		) : (
			<Text c={item.status === "failed" ? "red" : "dimmed"} size="sm">
				{description}
			</Text>
		);
	}
	return (
		<>
			<Stack flex={wide ? 1 : undefined} gap={0} mih={wide ? "70%" : undefined}>
				<BacktestChart
					backtest={result}
					enabled={enabled}
					fillHeight={wide}
					key={run.revision}
					paperPadding={paperPadding}
				/>
			</Stack>
			<BacktestResults
				backtest={result}
				gap={gap}
				paperPadding={paperPadding}
				period={
					run.parameters.from !== undefined || run.parameters.to !== undefined
				}
			/>
		</>
	);
}
