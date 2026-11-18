import { useSelector } from "@tanstack/react-form";
import equal from "fast-deep-equal";

import { criterionKeys } from "@/features/analysis/identifiers";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import {
	type ChartPriceLevels,
	createChartPriceLevels,
} from "@/features/candle-chart";
import { gridCalculatorLevels } from "@/features/instrument-analysis/grid-estimator/estimate";
import type { GridCalculatorFormValues } from "@/features/instrument-analysis/grid-estimator/types";
import { useGridCalculatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-calculator-form";
import { useChartIndicators } from "@/features/instrument-analysis/use-chart-indicators";
import { useCoinChartData } from "@/features/instrument-analysis/use-coin-chart-data";
import { useGridLimits } from "@/features/instrument-analysis/use-grid-limits";
import { useHourlyHistory } from "@/features/instrument-analysis/use-hourly-history";
import { useInstrumentAnalysis } from "@/features/instrument-analysis/use-instrument-analysis";
import { volatilityEvaluation } from "@/features/market-scan/criteria";
import { volatilityCriterionSelections } from "@/features/market-scan/pipeline";
import { useBusinessRequestPermission } from "@/utils/business-request-context";

// The calculator grid drawn on the chart, kept while its levels are equal.
function selectGridLevels({
	values,
}: {
	values: GridCalculatorFormValues;
}): ChartPriceLevels | null {
	const levels = gridCalculatorLevels(values);
	return levels && createChartPriceLevels(levels);
}

function sameGridLevels(
	a: ChartPriceLevels | null,
	b: ChartPriceLevels | null,
): boolean {
	return equal(a?.levels, b?.levels);
}

// The data of the coin page: the analysis of symbol under settings, the
// history and grid limits of the calculator, its form, the chart, and the
// calculator grid drawn on it. The page is built anew for another symbol.
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
	// Streamed by the page for the chart and the price alerts, which show its
	// current price even when the chart is not.
	const chartSource = useCoinChartData(
		symbol,
		indicators.catalogs,
		permission.allowed,
	);
	const current =
		analysis.result?.symbol === symbol ? analysis.result : undefined;
	const rangePercent = (key: string) => {
		const range = current && volatilityEvaluation(current.evaluations, key);
		return range && Number.isFinite(range.rangePercent)
			? range.rangePercent
			: undefined;
	};

	const dailyVolatilityPercent = rangePercent(criterionKeys.dailyVolatility);
	const hourlyVolatilityPercent = rangePercent(criterionKeys.hourlyVolatility);
	const calculatorReady = !hourlyHistory.pending && !gridLimits.pending;
	const calculatorForm = useGridCalculatorForm({
		candles: hourlyHistory.candles,
		dailyVolatilityPercent,
		gridLimits: gridLimits.limits,
		hourlyVolatilityPercent,
		// The ranges of other settings may already be cached, so the settings
		// identify them along with where the result comes from: the previous
		// one stays visible while refetching, and a failure has none.
		rangesKey: `${JSON.stringify(settings)}:${analysis.isPlaceholderData ? "previous" : current ? "current" : "none"}`,
		ready: calculatorReady,
	});
	const gridLevels = useSelector(calculatorForm.store, selectGridLevels, {
		compare: sameGridLevels,
	});

	return {
		allowed: permission.allowed,
		analysis,
		calculator: {
			dailyVolatilityPercent,
			disabled: analysis.isFetching || hourlyHistory.pending,
			form: calculatorForm,
			gridLimits: gridLimits.limits,
			hourlyVolatilityPercent,
		},
		// Shown once its prices are known.
		calculatorReady,
		chartSource,
		indicators,
		gridLevels: calculatorReady ? gridLevels : null,
	};
}
