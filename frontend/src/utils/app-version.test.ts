import { describe, expect, it } from "vitest";

import { getAppVersion } from "@/utils/app-version";

describe("getAppVersion", () => {
	it("uses dev when no build version is set", () => {
		expect(getAppVersion()).toBe("dev");
	});
});
