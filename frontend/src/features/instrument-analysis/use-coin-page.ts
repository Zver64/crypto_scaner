import { useState } from "react";

import { useBusinessRequestPermission } from "@/app/business-request-context";
import { criterionKeys } from "@/features/analysis/identifiers";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import {
	type ChartPriceLevels,
	createChartPriceLevels,
} from "@/features/candle-chart";
import { useChartIndicators } from "@/features/instrument-analysis/use-chart-indicators";
import { useCoinChartData } from "@/features/instrument-analysis/use-coin-chart-data";
import { useGridLimits } from "@/features/instrument-analysis/use-grid-limits";
import { useHourlyHistory } from "@/features/instrument-analysis/use-hourly-history";
import { useInstrumentAnalysis } from "@/features/instrument-analysis/use-instrument-analysis";
import { volatilityEvaluation } from "@/features/market-scan/criteria";
import { volatilityCriterionSelections } from "@/features/market-scan/pipeline";

// The data of the coin page: the analysis of symbol under settings, the
// history and grid limits of the calculator, the chart, and the calculator
// grid drawn on it.
export function useCoinPage(symbol: string, settings: VolatilitySettings) {
	const permission = useBusinessRequestPermission();
	const analysis = useInstrumentAnalysis(
		symbol,
		volatilityCriterionSelections(settings),
		permission.allowed,
	);
	const hourlyHistory = useHourlyHistory(symbol, permission.allowed);
	const gridLimits = useGridLimits(symbol, permission.allowed);
	const indicators = useChartIndicators(permission.allowed);
	// Shared by the chart and the price alerts, which show its current price.
	const chartSource = useCoinChartData(symbol, indicators.catalogs);
	// The calculator grid drawn on the chart, kept with its symbol: the page
	// stays mounted when the symbol changes.
	const [grid, setGrid] = useState<{
		levels: ChartPriceLevels | null;
		symbol: string;
	}>({ levels: null, symbol });

	// The previous result stays visible while refetching; the spot grid only
	// uses the result of this symbol and these settings, so the calculator is
	// built anew once they change.
	const current =
		analysis.result?.symbol === symbol ? analysis.result : undefined;
	const recommendationReady =
		current !== undefined &&
		!analysis.isPlaceholderData &&
		!hourlyHistory.pending &&
		!gridLimits.pending;
	const rangePercent = (key: string) => {
		const range = current && volatilityEvaluation(current.evaluations, key);
		return range && Number.isFinite(range.rangePercent)
			? range.rangePercent
			: undefined;
	};

	return {
		allowed: permission.allowed,
		analysis,
		calculator: {
			candles: recommendationReady ? hourlyHistory.candles : undefined,
			dailyVolatilityPercent: rangePercent(criterionKeys.dailyVolatility),
			disabled: analysis.isFetching || hourlyHistory.pending,
			gridLimits: recommendationReady ? gridLimits.limits : undefined,
			hourlyVolatilityPercent: rangePercent(criterionKeys.hourlyVolatility),
		},
		// The calculator is built anew once its recommendation is ready.
		calculatorKey: `binance:spot:USDT:${symbol}:${recommendationReady ? "ready" : "pending"}`,
		chartSource,
		indicators,
		gridLevels: grid.symbol === symbol ? grid.levels : null,
		setGridLevels: (levels: readonly number[] | null) =>
			setGrid({ levels: levels && createChartPriceLevels(levels), symbol }),
	};
}
