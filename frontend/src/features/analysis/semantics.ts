import type {
	CriterionRequest,
	Evaluation,
	MarketAnalysisItem,
	MarketAnalysisResponse,
} from "@/api/generated/models";
import {
	criterionNames,
	evaluationMetricKeys,
} from "@/features/analysis/identifiers";

export function hasExpectedInstrumentAnalysisEvaluations(
	evaluations: readonly Evaluation[],
	criteria: readonly CriterionRequest[],
): boolean {
	for (const criterion of criteria) {
		const evaluation = expectedEvaluation(evaluations, criterion);
		if (!evaluation) return false;
		if (!evaluation.matched) return true;
	}
	return true;
}

export function hasExpectedMarketScanResult(
	result: MarketAnalysisResponse,
	criteria: readonly CriterionRequest[],
): boolean {
	return (
		result.items.every((item) => hasExpectedEvaluations(item, criteria)) &&
		isSevenDayHourlyWindow(result)
	);
}

function hasExpectedEvaluations(
	item: MarketAnalysisItem,
	criteria: readonly CriterionRequest[],
): boolean {
	return (
		criteria.every(
			(criterion) =>
				expectedEvaluation(item.evaluations, criterion) !== undefined,
		) &&
		item.price_history.length === 169 &&
		item.price_history.every(
			(value) => value === null || Number.isFinite(value),
		)
	);
}

function isSevenDayHourlyWindow(result: MarketAnalysisResponse): boolean {
	const from = Date.parse(result.price_history_window.from);
	const to = Date.parse(result.price_history_window.to);
	return (
		Number.isFinite(from) &&
		Number.isFinite(to) &&
		from % 3_600_000 === 0 &&
		to % 3_600_000 === 0 &&
		to - from === 168 * 3_600_000
	);
}

function expectedEvaluation(
	evaluations: readonly Evaluation[],
	criterion: CriterionRequest,
): Evaluation | undefined {
	const evaluation = evaluations.find(({ key }) => key === criterion.key);
	return evaluation && isExpectedEvaluation(evaluation, criterion)
		? evaluation
		: undefined;
}

function isExpectedEvaluation(
	evaluation: Evaluation,
	criterion: CriterionRequest,
): boolean {
	const from = Date.parse(evaluation.from);
	const to = Date.parse(evaluation.to);
	return (
		evaluation.label === criterion.label &&
		evaluation.name === criterion.name &&
		Number.isInteger(evaluation.candle_count) &&
		evaluation.candle_count >= 0 &&
		Number.isFinite(from) &&
		Number.isFinite(to) &&
		to >= from &&
		hasExpectedMetrics(evaluation, criterion)
	);
}

function hasExpectedMetrics(
	evaluation: Evaluation,
	criterion: CriterionRequest,
): boolean {
	if (criterion.name === criterionNames.volatility) {
		return Number.isFinite(
			evaluation.metrics[evaluationMetricKeys.rangePercent],
		);
	}
	if (criterion.name === criterionNames.marketCap) {
		return Number.isFinite(
			evaluation.metrics[evaluationMetricKeys.marketCapUsd],
		);
	}
	return true;
}
