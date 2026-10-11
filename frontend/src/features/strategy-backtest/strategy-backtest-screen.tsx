import { Button, Paper, Stack, Text, Title } from "@mantine/core";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { EmptyState } from "@/components/empty-state";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { BacktestEvaluatedPeriod } from "@/features/strategy-backtest/backtest-evaluated-period";
import { BacktestMultiResults } from "@/features/strategy-backtest/backtest-multi-results";
import { BacktestPeriodFields } from "@/features/strategy-backtest/backtest-period-fields";
import { BacktestSingleResults } from "@/features/strategy-backtest/backtest-single-results";
import { CoinSelect } from "@/features/strategy-backtest/coin-select";
import { StrategySelect } from "@/features/strategy-backtest/strategy-select";
import { useBacktestRun } from "@/features/strategy-backtest/use-backtest-run";
import { formatNumber } from "@/utils/number-format";

const runHint = "Select coins and a strategy, then click Run backtest.";

interface StrategyBacktestScreenProps {
	allCoins: boolean;
	from: string | undefined;
	onCoinChange(symbols: string[], allCoins: boolean): void;
	onPeriodChange(from: string | undefined, to: string | undefined): void;
	onStrategyChange(strategy: number): void;
	strategy: number | undefined;
	symbols: string[];
	to: string | undefined;
}

export function StrategyBacktestScreen({
	allCoins,
	from,
	onCoinChange,
	onPeriodChange,
	onStrategyChange,
	strategy,
	symbols,
	to,
}: StrategyBacktestScreenProps) {
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	const permission = useBusinessRequestPermission();
	const wide = useWideLayout();
	const { run, running, start, cancel } = useBacktestRun();
	const periodError =
		from !== undefined && to !== undefined && from > to
			? "The period starts after it ends."
			: undefined;
	const canRun =
		permission.allowed &&
		strategy !== undefined &&
		symbols.length > 0 &&
		periodError === undefined &&
		!running;
	const multi = run ? run.parameters.symbols.length > 1 : symbols.length > 1;
	const singleResult =
		run?.parameters.symbols.length === 1 ? run.items[0].result : undefined;
	const finished =
		run?.items.filter(
			(item) => item.status === "completed" || item.status === "failed",
		).length ?? 0;

	const controls = (
		<Stack gap={contentSpacing}>
			<Title order={2} size="h4">
				{symbols.length === 1 ? symbols[0] : "Backtest"}
			</Title>
			<StrategySelect
				disabled={running}
				onChange={onStrategyChange}
				strategy={strategy}
			/>
			<CoinSelect
				allCoins={allCoins}
				disabled={running}
				onChange={onCoinChange}
				symbols={symbols}
			/>
			<BacktestPeriodFields
				disabled={running}
				error={periodError}
				from={from}
				onChange={onPeriodChange}
				to={to}
			/>
			<Button
				disabled={!canRun}
				loading={running}
				onClick={() => {
					if (canRun) void start({ strategy, symbols, from, to });
				}}
				size="sm"
				variant="light"
			>
				Run backtest
			</Button>
			{running ? (
				<>
					<Text c="dimmed" size="sm">
						Completed {finished} / {run?.items.length ?? symbols.length}
					</Text>
					<Button onClick={cancel} size="sm" variant="default">
						Cancel
					</Button>
				</>
			) : null}
			{singleResult && run ? (
				<Stack gap={4}>
					<BacktestEvaluatedPeriod
						backtest={singleResult}
						from={run.parameters.from}
						to={run.parameters.to}
					/>
					<Text c="dimmed" size="xs">
						Execution time:{" "}
						{formatNumber(
							singleResult.duration_ms / 1000,
							2,
							"halfExpand",
							false,
							2,
						)}{" "}
						s
					</Text>
				</Stack>
			) : null}
		</Stack>
	);
	return (
		<SidebarLayout
			gap={contentSpacing}
			sidebar={wide ? <Paper p={paperPadding}>{controls}</Paper> : null}
			sidebarPosition="end"
		>
			<Stack
				gap={contentSpacing}
				h={wide && !multi ? "100%" : undefined}
				style={wide && !multi ? { flexShrink: 0 } : undefined}
			>
				{wide ? null : controls}
				{run ? (
					multi ? (
						<BacktestMultiResults
							run={run}
							gap={contentSpacing}
							paperPadding={paperPadding}
						/>
					) : (
						<BacktestSingleResults
							run={run}
							gap={contentSpacing}
							paperPadding={paperPadding}
							wide={wide}
							enabled={permission.allowed}
						/>
					)
				) : wide ? (
					<EmptyState
						description={runHint}
						fillHeight={!multi}
						title="Run a backtest"
					/>
				) : (
					<Text c="dimmed" size="sm">
						{runHint}
					</Text>
				)}
			</Stack>
		</SidebarLayout>
	);
}
