import type { Evaluation } from "../../api/client";
import {
	marketCapEvaluation,
	percentileEvaluation,
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
	const percentile = percentileEvaluation(evaluations);
	return (
		percentile !== undefined &&
		(!marketCapRequired ||
			!percentile.matched ||
			marketCapEvaluation(evaluations) !== undefined)
	);
}
