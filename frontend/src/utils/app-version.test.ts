import { describe, expect, it } from "vitest";

import { getAppVersion } from "@/utils/app-version";

describe("getAppVersion", () => {
	it("uses the build version when it is set", () => {
		expect(getAppVersion("v1.2.3")).toBe("v1.2.3");
	});

	it("does not modify the build version", () => {
		expect(getAppVersion(" v1.2.3 ")).toBe(" v1.2.3 ");
	});

	it("uses dev when no build version is set", () => {
		expect(getAppVersion()).toBe("dev");
	});
});
