import { Button, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { useState } from "react";
import { useBacktestStrategy } from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { EmptyState } from "@/components/empty-state";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { BacktestChart } from "@/features/strategy-backtest/backtest-chart";
import { BacktestEvaluatedPeriod } from "@/features/strategy-backtest/backtest-evaluated-period";
import { BacktestPeriodFields } from "@/features/strategy-backtest/backtest-period-fields";
import { BacktestResults } from "@/features/strategy-backtest/backtest-results";
import { CoinSelect } from "@/features/strategy-backtest/coin-select";
import { StrategySelect } from "@/features/strategy-backtest/strategy-select";
import { backtestPeriod } from "@/features/strategy-backtest/utils";

const runHint = "Select a coin and a strategy, then click Run backtest.";
const runFailed = "The backtest could not be run.";

interface StrategyBacktestScreenProps {
	allCoins: boolean;
	// The first and last UTC days of the period, YYYY-MM-DD; absent for the
	// whole stored history on that side.
	from: string | undefined;
	onCoinChange(symbol: string | undefined, allCoins: boolean): void;
	onPeriodChange(from: string | undefined, to: string | undefined): void;
	onStrategyChange(strategy: number): void;
	strategy: number | undefined;
	symbol: string | undefined;
	to: string | undefined;
}

// Replays a strategy over the stored history of the coin, simulates its trades
// and marks them on the chart. The chart starts on the finest interval the strategy
// reads and offers the coarser ones.
export function StrategyBacktestScreen({
	allCoins,
	from,
	onCoinChange,
	onPeriodChange,
	onStrategyChange,
	strategy,
	symbol,
	to,
}: StrategyBacktestScreenProps) {
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	const permission = useBusinessRequestPermission();
	const wide = useWideLayout();
	const [requested, setRequested] = useState<
		| {
				from: string | undefined;
				strategy: number;
				symbol: string;
				to: string | undefined;
		  }
		| undefined
	>();
	const hasRun =
		requested !== undefined &&
		requested.strategy === strategy &&
		requested.symbol === symbol &&
		requested.from === from &&
		requested.to === to;
	const periodError =
		from !== undefined && to !== undefined && from > to
			? "The period starts after it ends."
			: undefined;
	const backtest = useBacktestStrategy(
		strategy ?? 0,
		{ symbol: symbol ?? "", chart: true, ...backtestPeriod(from, to) },
		{
			query: {
				// Only the Run button requests a backtest, including repeat runs.
				enabled: false,
				retry: false,
				select: (response) => response.data,
			},
		},
	);
	const canRun =
		permission.allowed &&
		strategy !== undefined &&
		symbol !== undefined &&
		periodError === undefined;

	const result = hasRun && !backtest.isError ? backtest.data : undefined;

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
			<BacktestPeriodFields
				error={periodError}
				from={from}
				onChange={onPeriodChange}
				to={to}
			/>
			<Button
				disabled={!canRun}
				loading={backtest.isFetching}
				onClick={() => {
					if (!canRun) return;
					setRequested({ from, strategy, symbol, to });
					void backtest.refetch();
				}}
				size="sm"
				variant="light"
			>
				Run backtest
			</Button>
			{result ? (
				<BacktestEvaluatedPeriod backtest={result} from={from} to={to} />
			) : null}
		</Stack>
	);

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
				) : result ? (
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
							<BacktestChart
								backtest={result}
								enabled={permission.allowed}
								fillHeight={wide}
								identity={backtest.dataUpdatedAt}
								paperPadding={paperPadding}
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
							period={from !== undefined || to !== undefined}
						/>
					</RefreshingOverlay>
				) : null}
			</Stack>
		</SidebarLayout>
	);
}
