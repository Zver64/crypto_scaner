import { describe, expect, it, vi } from "vitest";
import { fetchReadiness } from "@/api/readiness";

describe("fetchReadiness", () => {
	it("requests the relative readiness endpoint and reports success", async () => {
		const request = vi.fn(async () => new Response('{"status":"ready"}'));

		await expect(fetchReadiness(request)).resolves.toBe(true);
		expect(request).toHaveBeenCalledWith("/health/ready", {
			headers: { Accept: "application/json" },
			method: "GET",
		});
	});

	it("reports an unavailable HTTP response without exposing backend checks", async () => {
		const request = vi.fn(
			async () => new Response('{"status":"not_ready"}', { status: 503 }),
		);

		await expect(fetchReadiness(request)).resolves.toBe(false);
	});
});
