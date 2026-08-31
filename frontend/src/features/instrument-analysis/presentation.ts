import type { Evaluation } from "../../api/client";
import {
	marketCapEvaluation,
	volatilityEvaluation,
} from "../market-scan/criteria";

const utcDateFormatter = new Intl.DateTimeFormat("en", {
	dateStyle: "medium",
	timeZone: "UTC",
});

export function formatUtcCoverageDate(value: string): string {
	return `${utcDateFormatter.format(new Date(value))} UTC`;
}

export function hasRequiredInstrumentAnalysisEvaluations(
	evaluations: readonly Evaluation[],
	marketCapRequired: boolean,
): boolean {
	const volatility = volatilityEvaluation(evaluations);
	return (
		volatility !== undefined &&
		(!marketCapRequired ||
			!volatility.matched ||
			marketCapEvaluation(evaluations) !== undefined)
	);
}
