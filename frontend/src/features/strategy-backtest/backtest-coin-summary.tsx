import { Paper, Stack, Text, Title } from "@mantine/core";

import { BacktestEvaluatedPeriod } from "@/features/strategy-backtest/backtest-evaluated-period";
import { BacktestSignalSummary } from "@/features/strategy-backtest/backtest-signal-summary";
import { BacktestTradingSummary } from "@/features/strategy-backtest/backtest-trading-summary";
import type {
	BacktestCoinResult,
	BacktestRun,
} from "@/features/strategy-backtest/types";
import { noCandlesMessage } from "@/features/strategy-backtest/utils";
import { formatNumber } from "@/utils/number-format";

const statusLabels = {
	queued: "Queued",
	running: "Running backtest…",
	failed: "Backtest failed",
	cancelled: "Cancelled",
	not_run: "Not run: the batch stopped",
	completed: "Completed",
} satisfies Record<BacktestCoinResult["status"], string>;

interface BacktestCoinSummaryProps {
	item: BacktestCoinResult;
	run: BacktestRun;
	gap: string;
	paperPadding: string;
}

export function BacktestCoinSummary({
	item,
	run,
	gap,
	paperPadding,
}: BacktestCoinSummaryProps) {
	const result = item.result;
	return (
		<Paper p={paperPadding}>
			<Stack gap={gap}>
				<Title order={3} size="h4">
					{item.symbol}
				</Title>
				{result ? (
					<>
						<BacktestEvaluatedPeriod backtest={result} run={run} />
						<Text c="dimmed" size="xs">
							Execution time:{" "}
							{formatNumber(
								result.duration_ms / 1000,
								2,
								"halfExpand",
								false,
								2,
							)}{" "}
							s
						</Text>
						{result.from === null ? (
							<Text c="dimmed" size="sm">
								{noCandlesMessage(
									run.parameters.from !== undefined ||
										run.parameters.to !== undefined,
								)}
							</Text>
						) : result.signal ? (
							<BacktestSignalSummary signal={result.signal} />
						) : (
							<BacktestTradingSummary backtest={result} />
						)}
					</>
				) : (
					<Text c={item.status === "failed" ? "red" : "dimmed"} size="sm">
						{item.error ?? statusLabels[item.status]}
					</Text>
				)}
			</Stack>
		</Paper>
	);
}
