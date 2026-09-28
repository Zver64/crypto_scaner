import { expect, it } from "vitest";
import { rangeReadout } from "@/features/instrument-analysis/coin-chart-presentation";

it("formats the coin range metric", () => {
	expect(rangeReadout.format({ open: 1, high: 2, low: 0.5, close: 1.5 })).toBe(
		"150%",
	);
});
