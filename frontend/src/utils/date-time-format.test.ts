import { describe, expect, it } from "vitest";
import { formatDateTime } from "@/utils/date-time-format";

describe("formatDateTime", () => {
	it("formats a timestamp in UTC whatever its offset", () => {
		expect(formatDateTime("2026-10-07T23:30:00-02:00")).toBe(
			"Oct 8, 2026, 01:30 UTC",
		);
	});
});
