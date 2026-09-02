import { expect, it } from "vitest";
import {
	hasExpectedInstrumentAnalysisEvaluations,
	hasExpectedMarketScanEvaluations,
} from "@/api/analysis-contract";

const dailyCriterion = {
	key: "daily_volatility",
	label: "Daily Volatility",
	name: "volatility",
	parameters: {},
};
const hourlyCriterion = {
	...dailyCriterion,
	key: "hourly_volatility",
	label: "Hourly Volatility",
};
const marketCapCriterion = {
	key: "market_cap",
	label: "Market Cap",
	name: "market_cap",
	parameters: {},
};
const dailyEvaluation = (matched: boolean) => ({
	candle_count: 30,
	from: "2026-08-01T00:00:00Z",
	key: "daily_volatility",
	label: "Daily Volatility",
	matched,
	metrics: { range_percent: 4 },
	name: "volatility",
	to: "2026-08-02T00:00:00Z",
});
const hourlyEvaluation = (matched: boolean) => ({
	...dailyEvaluation(matched),
	candle_count: 60,
	key: "hourly_volatility",
	label: "Hourly Volatility",
});
const marketCapEvaluation = {
	candle_count: 0,
	from: "0001-01-01T00:00:00Z",
	key: "market_cap",
	label: "Market Cap",
	matched: true,
	metrics: { market_cap_usd: 1_000_000 },
	name: "market_cap",
	to: "0001-01-01T00:00:00Z",
};

it("requires every selected Market Scan evaluation", () => {
	const item = {
		matched: true,
		symbol: "BTCUSDT",
	};
	expect(
		hasExpectedMarketScanEvaluations(
			[
				{
					...item,
					evaluations: [dailyEvaluation(true), hourlyEvaluation(true)],
				},
			],
			[dailyCriterion, hourlyCriterion, marketCapCriterion],
		),
	).toBe(false);
	expect(
		hasExpectedMarketScanEvaluations(
			[
				{
					...item,
					evaluations: [
						dailyEvaluation(true),
						hourlyEvaluation(true),
						marketCapEvaluation,
					],
				},
			],
			[dailyCriterion, hourlyCriterion, marketCapCriterion],
		),
	).toBe(true);
});

it("allows Instrument Analysis to stop after an unmatched evaluation", () => {
	expect(
		hasExpectedInstrumentAnalysisEvaluations(
			[dailyEvaluation(false)],
			[dailyCriterion, hourlyCriterion, marketCapCriterion],
		),
	).toBe(true);
	expect(
		hasExpectedInstrumentAnalysisEvaluations(
			[dailyEvaluation(true)],
			[dailyCriterion, hourlyCriterion],
		),
	).toBe(false);
});
