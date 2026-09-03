import {
	criterionKeys,
	evaluationMetricKeys,
} from "@/api/analysis-identifiers";
import type { Evaluation } from "@/api/client";

const marketCapFormatter = new Intl.NumberFormat("en", {
	maximumFractionDigits: 1,
	notation: "compact",
});

export interface MarketCapEvaluation {
	marketCapUsd: number;
	matched: boolean;
}

export function formatMarketCapUsd(value: number): string {
	return `$${marketCapFormatter.format(value)}`;
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
