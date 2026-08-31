import type { Evaluation } from "../../api/client";
import {
	marketCapEvaluation,
	volatilityEvaluation,
} from "../market-scan/criteria";
import {
	dailyVolatilityKey,
	hourlyVolatilityKey,
} from "../market-scan/pipeline";

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
	const daily = volatilityEvaluation(evaluations, dailyVolatilityKey);
	const hourly = volatilityEvaluation(evaluations, hourlyVolatilityKey);
	return (
		daily !== undefined &&
		(!daily.matched ||
			(hourly !== undefined &&
				(!hourly.matched ||
					!marketCapRequired ||
					marketCapEvaluation(evaluations) !== undefined)))
	);
}
