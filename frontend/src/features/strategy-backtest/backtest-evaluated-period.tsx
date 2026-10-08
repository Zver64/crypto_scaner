import { Stack, Text } from "@mantine/core";
import type { StrategyBacktest } from "@/api/generated/models";
import { backtestDateFormat } from "@/features/strategy-backtest/constants";
import { shortenedPeriod } from "@/features/strategy-backtest/utils";
import { formatUtcDateTime } from "@/utils/date-time-format";

// The period days with the hour and minute of the candles.
const timeFormat = `${backtestDateFormat}, HH:mm`;

interface BacktestEvaluatedPeriodProps {
	backtest: StrategyBacktest;
	// The first and last UTC days chosen for the run, YYYY-MM-DD.
	from: string | undefined;
	to: string | undefined;
}

// The candles the backtest actually evaluated, in the date format of the
// period fields; nothing when it evaluated none. A period shorter than the
// chosen one turns yellow.
export function BacktestEvaluatedPeriod({
	backtest,
	from,
	to,
}: BacktestEvaluatedPeriodProps) {
	if (!backtest.from || !backtest.to) return null;
	const shortened = shortenedPeriod(
		{ from, to },
		{ from: backtest.from, interval: backtest.interval, to: backtest.to },
		Date.now(),
	);
	return (
		<Stack gap={2}>
			<Text c="dimmed" size="xs">
				Evaluated period
			</Text>
			<Text c={shortened.start || shortened.end ? "yellow" : "teal"} size="sm">
				{formatUtcDateTime(backtest.from, timeFormat)} →{" "}
				{formatUtcDateTime(backtest.to, timeFormat)}, {backtest.interval}
			</Text>
		</Stack>
	);
}
