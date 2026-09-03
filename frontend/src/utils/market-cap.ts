import type { Evaluation } from "@/api/client";

const marketCapCriterionName = "market_cap";

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
		({ key }) => key === marketCapCriterionName,
	);
	const marketCapUsd = evaluation?.metrics.market_cap_usd;
	if (!evaluation || typeof marketCapUsd !== "number") {
		return undefined;
	}

	return { marketCapUsd, matched: evaluation.matched };
}
