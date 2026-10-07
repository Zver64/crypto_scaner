import { Button, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import {
	type backtestStrategyResponseSuccess,
	getBacktestStrategyQueryKey,
	useBacktestStrategy,
} from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { EmptyState } from "@/components/empty-state";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { PriceHistoryChart } from "@/features/candle-chart";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { chartIntervals } from "@/features/instrument-analysis/live-candle-store";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { BacktestResults } from "@/features/strategy-backtest/backtest-results";
import { CoinSelect } from "@/features/strategy-backtest/coin-select";
import { HoldInput } from "@/features/strategy-backtest/hold-input";
import { StrategySelect } from "@/features/strategy-backtest/strategy-select";

// The backtest chart draws candles, volume and trades only.
const noIndicators = { "1h": [], "1d": [], "1w": [], "1M": [] } as const;
const runHint = "Select a coin and a strategy, then click Run backtest.";
const runFailed = "The backtest could not be run.";

interface StrategyBacktestScreenProps {
	allCoins: boolean;
	hold: number | undefined;
	onCoinChange(symbol: string | undefined, allCoins: boolean): void;
	onHoldChange(hold: number | undefined): void;
	onStrategyChange(strategy: number): void;
	strategy: number | undefined;
	symbol: string | undefined;
}

// Replays a strategy over the stored history of the coin, simulates its trades
// and marks them on the chart. The chart starts on the finest interval the strategy
// reads and offers the coarser ones.
export function StrategyBacktestScreen({
	allCoins,
	hold,
	onCoinChange,
	onHoldChange,
	onStrategyChange,
	strategy,
	symbol,
}: StrategyBacktestScreenProps) {
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	const permission = useBusinessRequestPermission();
	const wide = useWideLayout();
	const queryClient = useQueryClient();
	const [requested, setRequested] = useState<
		{ hold: number | undefined; strategy: number; symbol: string } | undefined
	>();
	const hasRun =
		requested !== undefined &&
		requested.strategy === strategy &&
		requested.symbol === symbol &&
		requested.hold === hold;
	const backtest = useBacktestStrategy(
		strategy ?? 0,
		{ hold, symbol: symbol ?? "" },
		{
			query: {
				// Only the Run button requests a backtest, including repeat runs.
				enabled: false,
				retry: false,
				select: (response) => response.data,
			},
		},
	);
	const source = useMemo(
		() => (symbol ? createCoinChartData(symbol, noIndicators) : undefined),
		[symbol],
	);
	// The strategy's interval and every coarser one, finest first.
	const intervals = useMemo(
		() =>
			backtest.data &&
			chartIntervals.slice(chartIntervals.indexOf(backtest.data.interval)),
		[backtest.data],
	);
	const tradeMarkers = useMemo(
		() =>
			backtest.data && {
				entries: backtest.data.trades.map(({ entry_time }) => entry_time),
				exits: backtest.data.trades.map(({ exit_time }) => exit_time),
			},
		[backtest.data],
	);
	const canRun =
		permission.allowed && strategy !== undefined && symbol !== undefined;
	// The default hold of the strategy's interval, once a run without a hold
	// on this coin has shown it.
	const defaultHold =
		strategy === undefined || symbol === undefined
			? undefined
			: queryClient.getQueryData<backtestStrategyResponseSuccess>(
					getBacktestStrategyQueryKey(strategy, { symbol }),
				)?.data.hold;

	const controls = (
		<Stack gap={contentSpacing}>
			<Title order={2} size="h4">
				{symbol ?? "Backtest"}
			</Title>
			<Group align="flex-start" grow wrap="nowrap">
				<StrategySelect onChange={onStrategyChange} strategy={strategy} />
				<CoinSelect
					allCoins={allCoins}
					onChange={onCoinChange}
					symbol={symbol}
				/>
			</Group>
			<Group align="flex-end" grow wrap="nowrap">
				<HoldInput
					defaultHold={defaultHold}
					hold={hold}
					onChange={onHoldChange}
				/>
				<Button
					disabled={!canRun}
					loading={backtest.isFetching}
					onClick={() => {
						if (!canRun) return;
						setRequested({ hold, strategy, symbol });
						void backtest.refetch();
					}}
					size="sm"
					variant="light"
				>
					Run backtest
				</Button>
			</Group>
		</Stack>
	);
	const result = hasRun && !backtest.isError ? backtest.data : undefined;

	// Wide screens keep the controls in the sidebar; phones show them above
	// the chart. Results follow the chart on both.
	return (
		<SidebarLayout
			gap={contentSpacing}
			sidebar={wide ? <Paper p={paperPadding}>{controls}</Paper> : null}
			sidebarPosition="end"
		>
			<Stack
				gap={contentSpacing}
				h={wide ? "100%" : undefined}
				style={wide ? { flexShrink: 0 } : undefined}
			>
				{wide ? null : controls}
				{/* Wide screens keep the chart's place with a card; phones show
				the plain hint under the controls. */}
				{strategy === undefined ||
				symbol === undefined ||
				!hasRun ||
				(backtest.isPending && !backtest.isFetching) ? (
					wide ? (
						<EmptyState
							description={runHint}
							fillHeight
							title="Run a backtest"
						/>
					) : (
						<Text c="dimmed" size="sm">
							{runHint}
						</Text>
					)
				) : backtest.isError ? (
					wide ? (
						<EmptyState
							description={backtest.error.info?.error.message}
							failed
							fillHeight
							title={runFailed}
						/>
					) : (
						<Text c="red" size="sm">
							{backtest.error.info?.error.message ?? runFailed}
						</Text>
					)
				) : result && source ? (
					<Stack
						flex={wide ? 1 : undefined}
						gap={0}
						mih={wide ? "70%" : undefined}
					>
						<RefreshingOverlay
							fillHeight={wide}
							label="Running the backtest"
							visible={backtest.isFetching}
						>
							<PriceHistoryChart
								enabled={permission.allowed}
								fillHeight={wide}
								indicators={noIndicators}
								intervals={intervals}
								key={`${symbol}:${strategy}:${result.interval}`}
								markers={tradeMarkers}
								paperPadding={paperPadding}
								source={source}
								symbol={symbol}
							/>
						</RefreshingOverlay>
					</Stack>
				) : (
					<CoinChartPlaceholder failed={false} paperPadding={paperPadding} />
				)}
				{result ? (
					<RefreshingOverlay
						label="Running the backtest"
						visible={backtest.isFetching}
					>
						<BacktestResults
							backtest={result}
							gap={contentSpacing}
							paperPadding={paperPadding}
						/>
					</RefreshingOverlay>
				) : null}
			</Stack>
		</SidebarLayout>
	);
}
