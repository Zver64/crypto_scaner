import { describe, expect, it } from "vitest";
import { getBusinessRequestPermission } from "@/utils/business-request-permission";

describe("getBusinessRequestPermission", () => {
	it("blocks production requests without signed Telegram init data", () => {
		expect(
			getBusinessRequestPermission({
				backendReady: true,
				isProduction: true,
				telegramInitData: "  ",
			}),
		).toEqual({ allowed: false, authenticated: false, backendReady: true });
	});

	it("allows an ordinary development browser when the backend is ready", () => {
		expect(
			getBusinessRequestPermission({
				backendReady: true,
				isProduction: false,
				telegramInitData: undefined,
			}),
		).toEqual({ allowed: true, authenticated: true, backendReady: true });
	});

	it("blocks authenticated requests while the backend is unavailable", () => {
		expect(
			getBusinessRequestPermission({
				backendReady: false,
				isProduction: true,
				telegramInitData: "query_id=signed",
			}),
		).toEqual({ allowed: false, authenticated: true, backendReady: false });
	});
});
