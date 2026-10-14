import { Paper, Stack, Text, useComputedColorScheme } from "@mantine/core";
import { LineSeries } from "lightweight-charts";
import { useLayoutEffect, useRef } from "react";
import type { StrategyBacktest } from "@/api/generated/models";
import {
	ChartCanvas,
	type ChartCanvasHandle,
	Series,
} from "@/components/lightweight-chart";
import { timeChartOptions } from "@/features/candle-chart/chart-options";
import {
	equityChartHeight,
	equityChartOptions,
	equitySeriesOptions,
} from "@/features/strategy-backtest/config";
import {
	createEquityData,
	formatEquityPercent,
} from "@/features/strategy-backtest/utils";

interface BacktestEquityChartProps {
	backtest: StrategyBacktest;
	paperPadding: string;
}

// Compounded net profit after each closed trade; shown only with trades. The
// equity has a point per closed trade, so the series compares it cheaply.
export function BacktestEquityChart({
	backtest,
	paperPadding,
}: BacktestEquityChartProps) {
	const { equity, from, interval } = backtest;
	const chartRef = useRef<ChartCanvasHandle>(null);
	const base = timeChartOptions[useComputedColorScheme("dark")][interval];
	// Fits each result once; runs after the series received its data.
	useLayoutEffect(() => {
		if (backtest.from) chartRef.current?.fitContent();
	}, [backtest]);

	return (
		<Paper component="section" p={paperPadding}>
			<Stack gap="xs">
				<Text fw={700}>Equity</Text>
				<ChartCanvas
					options={{
						...base,
						...equityChartOptions,
						localization: {
							...base.localization,
							priceFormatter: formatEquityPercent,
						},
						timeScale: { ...base.timeScale, ...equityChartOptions.timeScale },
					}}
					ref={chartRef}
					style={{ height: equityChartHeight, width: "100%" }}
				>
					<Series
						data={from ? createEquityData(from, equity) : []}
						definition={LineSeries}
						options={equitySeriesOptions}
					/>
				</ChartCanvas>
			</Stack>
		</Paper>
	);
}
