import { describe, expect, it } from "vitest";
import type { CriterionSelection } from "@/api/client";
import { marketScanQueryOptions } from "@/api/market-scan";

const criteria: readonly CriterionSelection[] = [
	{
		key: "market_cap",
		label: "Market Cap",
		name: "market_cap",
		parameters: { min_market_cap_usd: 0 },
	},
];

describe("marketScanQueryOptions", () => {
	it("keeps the existing Market Scan cache key unchanged", () => {
		expect(marketScanQueryOptions(criteria).queryKey).toEqual([
			"market-scan",
			criteria,
		]);
	});

	it("separates backend sort and limit requests from ordinary Market Scan", () => {
		const requestOptions = {
			limit: 10,
			sort: { direction: "desc", field: "market_cap_usd" },
		} as const;

		expect(marketScanQueryOptions(criteria, requestOptions).queryKey).toEqual([
			"market-scan",
			criteria,
			requestOptions,
		]);
	});
});
