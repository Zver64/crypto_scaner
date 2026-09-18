import { describe, expect, it } from "vitest";
import { createPageSearchMemory } from "@/app/page-search-memory";

describe("page search memory", () => {
	it("starts with no remembered searches", () => {
		const memory = createPageSearchMemory();

		expect(memory.recall("/")).toBeUndefined();
		expect(memory.recall("/top-coins")).toBeUndefined();
	});

	it("remembers page searches independently", () => {
		const memory = createPageSearchMemory();
		memory.remember("/", { symbol_filter: "BTC" });
		memory.remember("/top-coins", {
			hourly_percentile: 80,
			hourly_period: 24,
			percentile: 90,
			period: 30,
		});

		expect(memory.recall("/")).toEqual({ symbol_filter: "BTC" });
		expect(memory.recall("/top-coins")).toEqual({
			hourly_percentile: 80,
			hourly_period: 24,
			percentile: 90,
			period: 30,
		});
	});

	it("replaces an earlier search for the same page", () => {
		const memory = createPageSearchMemory();
		memory.remember("/", { symbol_filter: "BTC" });
		memory.remember("/", { symbol_filter: "ETH" });

		expect(memory.recall("/")).toEqual({ symbol_filter: "ETH" });
	});

	it("keeps overlapping sort keys isolated by page", () => {
		const memory = createPageSearchMemory();
		memory.remember("/", {
			sort_column: "marketCapUsd",
			sort_direction: "desc",
		});
		memory.remember("/top-coins", {
			hourly_percentile: 80,
			hourly_period: 24,
			percentile: 90,
			period: 30,
			sort_column: "dailyRangePercent",
			sort_direction: "asc",
		});

		expect(memory.recall("/")).toMatchObject({
			sort_column: "marketCapUsd",
			sort_direction: "desc",
		});
		expect(memory.recall("/top-coins")).toMatchObject({
			sort_column: "dailyRangePercent",
			sort_direction: "asc",
		});
	});

	it("copies searches on write and read", () => {
		const memory = createPageSearchMemory();
		const search = { symbol_filter: "BTC" };
		memory.remember("/", search);
		search.symbol_filter = "ETH";

		const recalled = memory.recall("/");
		expect(recalled).toEqual({ symbol_filter: "BTC" });

		if (recalled) {
			recalled.symbol_filter = "SOL";
		}
		expect(memory.recall("/")).toEqual({ symbol_filter: "BTC" });
	});

	it("does not carry searches into a fresh instance", () => {
		const existingMemory = createPageSearchMemory();
		existingMemory.remember("/", { symbol_filter: "BTC" });

		const freshMemory = createPageSearchMemory();
		expect(freshMemory.recall("/")).toBeUndefined();
	});
});
