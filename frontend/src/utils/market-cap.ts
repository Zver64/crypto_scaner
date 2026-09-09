import {
	criterionKeys,
	evaluationMetricKeys,
} from "@/api/analysis-identifiers";
import type { Evaluation } from "@/api/client";
import { formatCompactNumber } from "@/utils/number-format";

export interface MarketCapEvaluation {
	marketCapUsd: number;
	matched: boolean;
}

export function formatMarketCapUsd(value: number): string {
	return `$${formatCompactNumber(value)}`;
}

export function marketCapEvaluation(
	evaluations: readonly Evaluation[],
): MarketCapEvaluation | undefined {
	const evaluation = evaluations.find(
		({ key }) => key === criterionKeys.marketCap,
	);
	const marketCapUsd = evaluation?.metrics[evaluationMetricKeys.marketCapUsd];
	if (!evaluation || typeof marketCapUsd !== "number") {
		return undefined;
	}

	return { marketCapUsd, matched: evaluation.matched };
}
