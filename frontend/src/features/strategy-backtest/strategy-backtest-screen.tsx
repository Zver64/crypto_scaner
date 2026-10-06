import { Button, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { keepPreviousData } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useBacktestStrategy } from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { hasTelegramBackButton } from "@/app/telegram";
import { EmptyState } from "@/components/empty-state";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { PriceHistoryChart } from "@/features/candle-chart";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { chartIntervals } from "@/features/instrument-analysis/live-candle-store";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { BacktestAlerts } from "@/features/strategy-backtest/backtest-alerts";
import { StrategySelect } from "@/features/strategy-backtest/strategy-select";
import { useBackToBacktestCoins } from "@/features/strategy-backtest/use-back-to-backtest-coins";

// The backtest chart draws candles, volume and alerts only.
const noIndicators = { "1h": [], "1d": [], "1w": [], "1M": [] } as const;

interface StrategyBacktestScreenProps {
	onStrategyChange(strategy: number): void;
	strategy: number | undefined;
	symbol: string;
}

// Replays a strategy over the stored history of the coin and marks where it
// would have alerted. The chart starts on the finest interval the strategy
// reads and offers the coarser ones.
export function StrategyBacktestScreen({
	onStrategyChange,
	strategy,
	symbol,
}: StrategyBacktestScreenProps) {
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	const permission = useBusinessRequestPermission();
	const backToCoins = useBackToBacktestCoins();
	const wide = useWideLayout();
	const backtest = useBacktestStrategy(
		strategy ?? 0,
		{ symbol: symbol.toUpperCase() },
		{
			query: {
				enabled: permission.allowed && strategy !== undefined,
				// The previous result stays on screen while another strategy is
				// replayed.
				placeholderData: keepPreviousData,
				retry: false,
				select: (response) => response.data,
			},
		},
	);
	// The strategy whose own result is on screen; the chart restarts on its
	// interval only once that result arrives.
	const [shownStrategy, setShownStrategy] = useState(strategy);
	if (
		backtest.data &&
		!backtest.isPlaceholderData &&
		shownStrategy !== strategy
	)
		setShownStrategy(strategy);
	const source = useMemo(
		() => createCoinChartData(symbol, noIndicators),
		[symbol],
	);
	// The strategy's interval and every coarser one, finest first.
	const intervals = useMemo(
		() =>
			backtest.data &&
			chartIntervals.slice(chartIntervals.indexOf(backtest.data.interval)),
		[backtest.data],
	);
	const alertTimes = useMemo(
		() => backtest.data?.alerts.map(({ open_time }) => open_time),
		[backtest.data],
	);

	const controls = (
		<Stack gap={contentSpacing}>
			<Group justify="space-between">
				<Title order={2} size="h4">
					{symbol.toUpperCase()}
				</Title>
				{/* Telegram's native back button returns to the list otherwise. */}
				{backToCoins && !hasTelegramBackButton() ? (
					<Button onClick={backToCoins} size="sm" variant="subtle">
						Back to coins
					</Button>
				) : null}
			</Group>
			<StrategySelect onChange={onStrategyChange} strategy={strategy} />
		</Stack>
	);
	const result =
		strategy !== undefined && !backtest.isError ? backtest.data : undefined;

	// Wide screens fill the height with the chart and keep the controls and
	// alerts in the sidebar; phones show the controls above the chart.
	return (
		<SidebarLayout
			gap={contentSpacing}
			sidebar={
				<Stack gap={contentSpacing}>
					{wide ? <Paper p={paperPadding}>{controls}</Paper> : null}
					{result ? (
						<RefreshingOverlay
							label="Running the backtest"
							visible={backtest.isPlaceholderData}
						>
							<BacktestAlerts backtest={result} paperPadding={paperPadding} />
						</RefreshingOverlay>
					) : null}
				</Stack>
			}
			sidebarPosition="end"
		>
			<Stack flex={wide ? 1 : undefined} gap={contentSpacing}>
				{wide ? null : controls}
				{/* Wide screens keep the chart's place with a card; phones show
				the plain hint under the controls. */}
				{strategy === undefined ? (
					wide ? (
						<EmptyState
							description="Pick one in the sidebar to see where it would have alerted on this coin."
							fillHeight
							title="Choose a strategy"
						/>
					) : (
						<Text c="dimmed" size="sm">
							Choose a strategy to see where it would have alerted.
						</Text>
					)
				) : backtest.isError ? (
					wide ? (
						<EmptyState
							description={backtest.error.info?.error.message}
							failed
							fillHeight
							title="The backtest could not be run."
						/>
					) : (
						<Text c="red" size="sm">
							{backtest.error.info?.error.message ??
								"The backtest could not be run."}
						</Text>
					)
				) : result ? (
					<RefreshingOverlay
						fillHeight={wide}
						label="Running the backtest"
						visible={backtest.isPlaceholderData}
					>
						<PriceHistoryChart
							enabled={permission.allowed}
							fillHeight={wide}
							indicators={noIndicators}
							intervals={intervals}
							key={`${symbol}:${shownStrategy}`}
							markers={alertTimes}
							paperPadding={paperPadding}
							source={source}
							symbol={symbol}
						/>
					</RefreshingOverlay>
				) : (
					<CoinChartPlaceholder failed={false} paperPadding={paperPadding} />
				)}
			</Stack>
		</SidebarLayout>
	);
}
