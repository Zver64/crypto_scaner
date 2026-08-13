import { resolveConfig } from "vite";
import { describe, expect, it } from "vitest";
import { developmentAuthorizationHeader } from "../vite.config";

describe("developmentAuthorizationHeader", () => {
	it("omits authorization for missing or blank init data", () => {
		expect(developmentAuthorizationHeader(undefined)).toBeUndefined();
		expect(developmentAuthorizationHeader("  ")).toBeUndefined();
	});

	it("uses the exact tma scheme for configured fake init data", () => {
		expect(developmentAuthorizationHeader(" signed-fixture ")).toBe(
			"tma signed-fixture",
		);
	});
});

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
