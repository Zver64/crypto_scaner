import { useState } from "react";
import type { StrategyBacktest } from "@/api/generated/models";
import { PriceHistoryChart } from "@/features/candle-chart";
import { createBacktestChart } from "@/features/strategy-backtest/chart-data";

interface BacktestChartProps {
	backtest: StrategyBacktest;
	enabled: boolean;
	fillHeight: boolean;
	paperPadding: string;
}

// Chart preparation belongs to the completed run, not to its request controls.
// The parent keys the chart by run, so every run, even one with a result equal
// to the previous one, gets a new snapshot source and a reset viewport.
export function BacktestChart({
	backtest,
	enabled,
	fillHeight,
	paperPadding,
}: BacktestChartProps) {
	// The source holds the revealed history of this run's chart.
	const [chart] = useState(() => createBacktestChart(backtest));
	return (
		<PriceHistoryChart
			enabled={enabled}
			fillHeight={fillHeight}
			indicators={chart.indicators}
			intervals={chart.intervals}
			live={false}
			paperPadding={paperPadding}
			source={chart.source}
			symbol={backtest.symbol}
		/>
	);
}
