import { resolveConfig } from "vite";
import { describe, expect, it } from "vitest";

describe("Vite dependency optimization", () => {
	it("prebundles route-only Mantine Form before the first browser request", async () => {
		const config = await resolveConfig(
			{ logLevel: "silent" },
			"serve",
			"development",
		);

		expect(config.optimizeDeps.include).toContain("@mantine/form");
	});
});
