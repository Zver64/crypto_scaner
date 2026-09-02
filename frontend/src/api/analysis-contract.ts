import type {
	CriterionSelection,
	Evaluation,
	MarketScanItem,
} from "@/api/client";

export function hasExpectedInstrumentAnalysisEvaluations(
	evaluations: readonly Evaluation[],
	criteria: readonly CriterionSelection[],
): boolean {
	for (const criterion of criteria) {
		const evaluation = expectedEvaluation(evaluations, criterion);
		if (!evaluation) return false;
		if (!evaluation.matched) return true;
	}
	return true;
}

export function hasExpectedMarketScanEvaluations(
	items: readonly MarketScanItem[],
	criteria: readonly CriterionSelection[],
): boolean {
	return items.every((item) =>
		criteria.every(
			(criterion) =>
				expectedEvaluation(item.evaluations, criterion) !== undefined,
		),
	);
}

function expectedEvaluation(
	evaluations: readonly Evaluation[],
	criterion: CriterionSelection,
): Evaluation | undefined {
	const evaluation = evaluations.find(({ key }) => key === criterion.key);
	return evaluation && hasExpectedMetrics(evaluation, criterion)
		? evaluation
		: undefined;
}

function hasExpectedMetrics(
	evaluation: Evaluation,
	criterion: CriterionSelection,
): boolean {
	if (criterion.name === "volatility") {
		return typeof evaluation.metrics.range_percent === "number";
	}
	if (criterion.name === "market_cap") {
		return typeof evaluation.metrics.market_cap_usd === "number";
	}
	return true;
}
