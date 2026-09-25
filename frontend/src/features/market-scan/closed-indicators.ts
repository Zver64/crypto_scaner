import type { CandleInterval, ClosedIndicator } from "@/api/generated/models";

export interface ClosedIndicatorOutputKey {
	type: string;
	interval: CandleInterval;
	parameters: Record<string, unknown>;
	output: string;
}

export const dailyRsi14: ClosedIndicatorOutputKey = {
	type: "rsi",
	interval: "1d",
	parameters: { period: 14 },
	output: "rsi",
};

export function closedIndicatorValue(
	indicators: readonly ClosedIndicator[],
	key: ClosedIndicatorOutputKey,
): number | null {
	const parameters = canonicalParameters(key.parameters);
	const indicator = indicators.find(
		(item) =>
			item.type === key.type &&
			item.interval === key.interval &&
			canonicalParameters(item.parameters) === parameters,
	);
	return (
		indicator?.outputs.find((output) => output.name === key.output)?.value ??
		null
	);
}

function canonicalParameters(parameters: Record<string, unknown>): string {
	return JSON.stringify(parameters, Object.keys(parameters).sort());
}
