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
