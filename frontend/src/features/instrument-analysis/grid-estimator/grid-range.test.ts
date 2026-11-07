import { describe, expect, it } from "vitest";

import {
	gridCountForRange,
	gridCountForStep,
	gridCountStepError,
	gridCountStepPercent,
} from "@/features/instrument-analysis/grid-estimator/grid-range";

describe("grid count step", () => {
	const range = { gridCount: "4", lowerPrice: "90", upperPrice: "110" };

	it("does not depend on the investment", () => {
		expect(gridCountStepPercent(range, "geometric")).toBeGreaterThan(5);
	});

	it("rejects a step outside the hourly-to-daily range", () => {
		expect(gridCountStepError(2, 1, 3)).toBeNull();
		expect(gridCountStepError(0.5, 1, 3)).toContain("from 1% to 3%");
		expect(gridCountStepError(4, 1, null)).toBeNull();
		expect(gridCountStepError(0.5, 1, null)).toContain("at least 1%");
		expect(gridCountStepError(0.5, null, 3)).toBeNull();
	});
});

describe("gridCountForRange", () => {
	const range = {
		gridType: "geometric",
		lowerPrice: "1230",
		rangePercent: 1,
		upperPrice: "1640",
	} as const;

	it("derives the count of a range wide enough for one step", () => {
		expect(gridCountForRange(range, "40", null, 0)).toEqual({
			error: null,
			gridCount: gridCountForStep("1640", "1230", 1, "geometric"),
		});
	});

	it("keeps the count and explains a range narrower than one step", () => {
		expect(
			gridCountForRange({ ...range, lowerPrice: "1639" }, "40", null, 0),
		).toEqual({
			error:
				"The price range is narrower than one 1% step, so the grid count is unchanged",
			gridCount: "40",
		});
	});

	it("keeps the count without an error for an invalid range or no step", () => {
		expect(
			gridCountForRange({ ...range, upperPrice: "" }, "40", null, 0),
		).toEqual({ error: null, gridCount: "40" });
		expect(
			gridCountForRange({ ...range, rangePercent: 0 }, "40", null, 0),
		).toEqual({ error: null, gridCount: "40" });
	});
});
