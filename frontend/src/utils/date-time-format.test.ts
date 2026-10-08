import { describe, expect, it } from "vitest";
import { formatDateTime, formatUtcDateTime } from "@/utils/date-time-format";

describe("formatDateTime", () => {
	it("formats a timestamp in UTC whatever its offset", () => {
		expect(formatDateTime("2026-10-07T23:30:00-02:00")).toBe(
			"Oct 8, 2026, 01:30 UTC",
		);
	});
});

describe("formatUtcDateTime", () => {
	it("formats a timestamp in UTC whatever its offset", () => {
		expect(
			formatUtcDateTime("2026-08-04T18:00:00+02:00", "DD.MM.YYYY, HH:mm"),
		).toBe("04.08.2026, 16:00 UTC");
	});
});
