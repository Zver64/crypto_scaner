import { describe, expect, it } from "vitest";
import {
	createDevelopmentInitData,
	parsePositiveInteger,
	requireEnvironmentValue,
	upsertEnvironmentValue,
} from "./dev-init-data.mjs";

describe("development init-data environment", () => {
	it("requires a non-blank configured value", () => {
		expect(requireEnvironmentValue({ TOKEN: " fake-token " }, "TOKEN")).toBe(
			"fake-token",
		);
		expect(() => requireEnvironmentValue({}, "TOKEN")).toThrow(
			"TOKEN is required in the repository root .env file",
		);
		expect(() => requireEnvironmentValue({ TOKEN: "  " }, "TOKEN")).toThrow(
			"TOKEN is required in the repository root .env file",
		);
	});

	it("accepts only positive safe Telegram IDs", () => {
		expect(parsePositiveInteger("424242", "ADMIN_TELEGRAM_ID")).toBe(424242);
		for (const value of ["0", "-1", "1.5", "abc", "9007199254740992"]) {
			expect(() => parsePositiveInteger(value, "ADMIN_TELEGRAM_ID")).toThrow(
				"ADMIN_TELEGRAM_ID must be a positive",
			);
		}
	});
});

describe("createDevelopmentInitData", () => {
	it("signs with the current authentication date supplied by the clock", () => {
		const currentDate = new Date("2026-08-14T01:00:00Z");
		const calls = [];
		const value = createDevelopmentInitData({
			botToken: "fake-token",
			now: () => currentDate,
			signInitData: (...args) => {
				calls.push(args);
				return "fake-signed-value";
			},
			telegramID: 424242,
		});

		expect(value).toBe("fake-signed-value");
		expect(calls).toEqual([
			[
				{
					user: {
						first_name: "Development",
						id: 424242,
						language_code: "en",
						last_name: "User",
					},
				},
				"fake-token",
				currentDate,
			],
		]);
	});
});

describe("upsertEnvironmentValue", () => {
	it("preserves unrelated entries while replacing the private credential", () => {
		expect(
			upsertEnvironmentValue(
				"KEEP=value\nTELEGRAM_DEV_INIT_DATA=old-fake\nAFTER=present\n",
				"TELEGRAM_DEV_INIT_DATA",
				"new-fake",
			),
		).toBe(
			"KEEP=value\nTELEGRAM_DEV_INIT_DATA=new-fake\nAFTER=present\n",
		);
	});

	it("adds the credential to empty and populated environment files", () => {
		expect(
			upsertEnvironmentValue("", "TELEGRAM_DEV_INIT_DATA", "fake"),
		).toBe("TELEGRAM_DEV_INIT_DATA=fake\n");
		expect(
			upsertEnvironmentValue(
				"KEEP=value\n",
				"TELEGRAM_DEV_INIT_DATA",
				"fake",
			),
		).toBe("KEEP=value\nTELEGRAM_DEV_INIT_DATA=fake\n");
	});
});
