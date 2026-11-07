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

	const current =
		analysis.result?.symbol === symbol ? analysis.result : undefined;
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
			candles: hourlyHistory.candles,
			dailyVolatilityPercent: rangePercent(criterionKeys.dailyVolatility),
			disabled: analysis.isFetching || hourlyHistory.pending,
			// The ranges of other settings may already be cached, so the settings
			// identify them along with where the result comes from: the previous
			// one stays visible while refetching, and a failure has none.
			formKey: `${JSON.stringify(settings)}:${analysis.isPlaceholderData ? "previous" : current ? "current" : "none"}`,
			gridLimits: gridLimits.limits,
			hourlyVolatilityPercent: rangePercent(criterionKeys.hourlyVolatility),
		},
		// Built once its prices are known, and anew for another symbol.
		calculatorKey: `binance:spot:USDT:${symbol}`,
		calculatorReady: !hourlyHistory.pending && !gridLimits.pending,
		chartSource,
		indicators,
		gridLevels: grid.symbol === symbol ? grid.levels : null,
		setGridLevels: (levels: readonly number[] | null) =>
			setGrid({ levels: levels && createChartPriceLevels(levels), symbol }),
	};
}
