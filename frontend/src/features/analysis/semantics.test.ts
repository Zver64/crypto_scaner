import { expect, it } from "vitest";
import type {
	CriterionRequest,
	Evaluation,
	MarketAnalysisResponse,
} from "@/api/generated/models";
import {
	hasExpectedInstrumentAnalysisEvaluations,
	hasExpectedMarketScanResult,
} from "@/features/analysis/semantics";

const dailyCriterion: CriterionRequest = {
	key: "daily_volatility",
	label: "Daily Volatility",
	name: "volatility",
	parameters: {},
};
const hourlyCriterion: CriterionRequest = {
	...dailyCriterion,
	key: "hourly_volatility",
	label: "Hourly Volatility",
};
const marketCapCriterion: CriterionRequest = {
	key: "market_cap",
	label: "Market Cap",
	name: "market_cap",
	parameters: {},
};
const dailyEvaluation = (matched: boolean): Evaluation => ({
	candle_count: 30,
	from: "2026-08-01T00:00:00Z",
	key: "daily_volatility",
	label: "Daily Volatility",
	matched,
	metrics: { range_percent: 4 },
	name: "volatility",
	to: "2026-08-02T00:00:00Z",
});
const hourlyEvaluation = (matched: boolean): Evaluation => ({
	...dailyEvaluation(matched),
	candle_count: 60,
	key: "hourly_volatility",
	label: "Hourly Volatility",
});
const marketCapEvaluation: Evaluation = {
	candle_count: 0,
	from: "0001-01-01T00:00:00Z",
	key: "market_cap",
	label: "Market Cap",
	matched: true,
	metrics: { market_cap_usd: 1_000_000 },
	name: "market_cap",
	to: "0001-01-01T00:00:00Z",
};

function marketResult(evaluations: Evaluation[]): MarketAnalysisResponse {
	return {
		analyzed_count: 1,
		insufficient_data_count: 0,
		items: [
			{
				evaluations,
				closed_indicators: [],
				matched: true,
				price_history: Array(169).fill(null),
				symbol: "BTCUSDT",
			},
		],
		matched_count: 1,
		price_history_window: {
			from: "2026-08-01T00:00:00Z",
			to: "2026-08-08T00:00:00Z",
		},
		unresolved: [],
		warnings: [],
	};
}

it("requires every selected Market Scan evaluation", () => {
	expect(
		hasExpectedMarketScanResult(
			marketResult([dailyEvaluation(true), hourlyEvaluation(true)]),
			[dailyCriterion, hourlyCriterion, marketCapCriterion],
		),
	).toBe(false);
	expect(
		hasExpectedMarketScanResult(
			marketResult([
				dailyEvaluation(true),
				hourlyEvaluation(true),
				marketCapEvaluation,
			]),
			[dailyCriterion, hourlyCriterion, marketCapCriterion],
		),
	).toBe(true);
});

it("rejects malformed market history", () => {
	const evaluations = [
		dailyEvaluation(true),
		hourlyEvaluation(true),
		marketCapEvaluation,
	];
	const missingPrice = marketResult(evaluations);
	missingPrice.items[0].price_history.pop();
	const invalidWindow = marketResult(evaluations);
	invalidWindow.price_history_window.from = "invalid";

	expect(
		hasExpectedMarketScanResult(missingPrice, [
			dailyCriterion,
			hourlyCriterion,
			marketCapCriterion,
		]),
	).toBe(false);
	expect(
		hasExpectedMarketScanResult(invalidWindow, [
			dailyCriterion,
			hourlyCriterion,
			marketCapCriterion,
		]),
	).toBe(false);
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
