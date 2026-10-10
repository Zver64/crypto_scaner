import { useComputedColorScheme, useMantineTheme } from "@mantine/core";
import { useMemo } from "react";
import type { CandleInterval, StrategyBacktest } from "@/api/generated/models";
import { PriceHistoryChart } from "@/features/candle-chart";
import { toChartIndicatorOptions } from "@/features/candle-chart/utils";
import { createBacktestChartData } from "@/features/strategy-backtest/chart-data";
import { backtestMarkers } from "@/features/strategy-backtest/utils";

interface BacktestChartProps {
	backtest: StrategyBacktest;
	enabled: boolean;
	fillHeight: boolean;
	identity: number;
	paperPadding: string;
}

// Chart preparation belongs to the completed run, not to its request controls.
export function BacktestChart({
	backtest,
	enabled,
	fillHeight,
	identity,
	paperPadding,
}: BacktestChartProps) {
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	// A successful response replaces the source even when Query structurally
	// shares identical data. Reset the viewport with the same snapshot identity.
	const snapshot = useMemo(
		() => ({
			identity,
			source: createBacktestChartData(backtest.charts),
		}),
		[backtest, identity],
	);
	const intervals = useMemo(
		() => backtest.charts.map(({ page }) => page.interval),
		[backtest],
	);
	const indicators = useMemo(() => {
		const result: Record<
			CandleInterval,
			ReturnType<typeof toChartIndicatorOptions>[]
		> = {
			"1h": [],
			"1d": [],
			"1w": [],
			"1M": [],
		};
		for (const chart of backtest.charts) {
			result[chart.page.interval] = chart.catalog.map((definition) =>
				toChartIndicatorOptions(definition, theme, colorScheme),
			);
		}
		return result;
	}, [backtest, theme, colorScheme]);
	const markers = useMemo(() => backtestMarkers(backtest), [backtest]);
	return (
		<PriceHistoryChart
			enabled={enabled}
			fillHeight={fillHeight}
			indicators={indicators}
			intervals={intervals}
			key={`${backtest.symbol}:${backtest.interval}:${snapshot.identity}`}
			live={false}
			markers={markers}
			paperPadding={paperPadding}
			source={snapshot.source}
			symbol={backtest.symbol}
		/>
	);
}
