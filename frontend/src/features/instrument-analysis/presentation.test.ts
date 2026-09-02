import { describe, expect, it } from "vitest";
import { formatUtcCoverageDate } from "./presentation";

describe("formatUtcCoverageDate", () => {
	it("renders the UTC calendar date without local-time conversion", () => {
		expect(formatUtcCoverageDate("2026-07-05T00:00:00Z")).toBe(
			"Jul 5, 2026 UTC",
		);
		expect(formatUtcCoverageDate("2026-08-03T23:59:59Z")).toBe(
			"Aug 3, 2026 UTC",
		);
	});
});
