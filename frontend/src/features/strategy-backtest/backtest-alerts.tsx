import { List, Paper, Stack, Text } from "@mantine/core";
import type { StrategyBacktest } from "@/api/generated/models";
import { formatChartTime } from "@/components/price-history-chart/utils";

interface BacktestAlertsProps {
	backtest: StrategyBacktest;
	paperPadding: string;
}

// The alerts of a backtest, newest first, and the stored history they cover.
export function BacktestAlerts({
	backtest: { alerts, from, interval, to },
	paperPadding,
}: BacktestAlertsProps) {
	return (
		<Paper component="section" p={paperPadding}>
			<Stack gap="xs">
				<Text fw={700}>
					{alerts.length === 1 ? "1 alert" : `${alerts.length} alerts`}
				</Text>
				<Text c="dimmed" size="sm">
					{from && to
						? `Covered: ${formatChartTime(from, interval)} – ${formatChartTime(to, interval)}`
						: "No stored history covers this strategy."}
				</Text>
				{alerts.length > 0 ? (
					<>
						<Text c="dimmed" size="sm">
							Each alert fires at the close of the candle opening at this time.
						</Text>
						<List size="sm" spacing={4}>
							{[...alerts].reverse().map(({ open_time }) => (
								<List.Item key={open_time}>
									{formatChartTime(open_time, interval)}
								</List.Item>
							))}
						</List>
					</>
				) : null}
			</Stack>
		</Paper>
	);
}
