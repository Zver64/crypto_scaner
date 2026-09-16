import type { Evaluation } from "@/api/generated/models";
import {
	criterionKeys,
	evaluationMetricKeys,
} from "@/features/analysis/identifiers";
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
