import { Paper, Stack, Text, useComputedColorScheme } from "@mantine/core";
import {
	type DeepPartial,
	LineSeries,
	type Time,
	type TimeChartOptions,
} from "lightweight-charts";
import { useLayoutEffect, useMemo, useRef } from "react";
import type { StrategyBacktest } from "@/api/generated/models";
import {
	ChartCanvas,
	type ChartCanvasHandle,
	Series,
} from "@/components/lightweight-chart";
import {
	chartOptions as baseChartOptions,
	chartIntervalOptions,
} from "@/features/candle-chart/config";
import { formatChartTime } from "@/features/candle-chart/utils";
import {
	equityChartHeight,
	equityChartOptions,
	equitySeriesOptions,
} from "@/features/strategy-backtest/config";
import { createEquityData } from "@/features/strategy-backtest/utils";
import { formatRangePercent } from "@/utils/range-percent";

interface BacktestEquityChartProps {
	backtest: StrategyBacktest;
	paperPadding: string;
}

// Compounded net profit after each closed trade; shown only with trades.
export function BacktestEquityChart({
	backtest: { equity, from, interval },
	paperPadding,
}: BacktestEquityChartProps) {
	const colorScheme = useComputedColorScheme("dark");
	const chartRef = useRef<ChartCanvasHandle>(null);
	const data = useMemo(
		() => (from ? createEquityData(from, equity) : []),
		[equity, from],
	);
	const options = useMemo<DeepPartial<TimeChartOptions>>(
		() => ({
			...baseChartOptions[colorScheme],
			...equityChartOptions,
			localization: {
				priceFormatter: (value: number) => formatRangePercent(value, 2),
				timeFormatter: (time: Time) =>
					typeof time === "number"
						? formatChartTime(time, interval)
						: String(time),
			},
			timeScale: {
				...baseChartOptions[colorScheme].timeScale,
				...equityChartOptions.timeScale,
				timeVisible:
					chartIntervalOptions.find((item) => item.value === interval)
						?.showTime ?? false,
			},
		}),
		[colorScheme, interval],
	);
	// Runs after the series received the data.
	useLayoutEffect(() => {
		if (data.length > 0) chartRef.current?.fitContent();
	}, [data]);

	return (
		<Paper component="section" p={paperPadding}>
			<Stack gap="xs">
				<Text fw={700}>Equity</Text>
				<ChartCanvas
					options={options}
					ref={chartRef}
					style={{ height: equityChartHeight, width: "100%" }}
				>
					<Series
						data={data}
						definition={LineSeries}
						options={equitySeriesOptions}
					/>
				</ChartCanvas>
			</Stack>
		</Paper>
	);
}
