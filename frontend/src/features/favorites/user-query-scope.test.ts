import { describe, expect, it } from "vitest";
import {
	scopedUserQueryKey,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";

describe("telegramUserScope", () => {
	it("uses only the Telegram user ID and never embeds init data", () => {
		const initData = new URLSearchParams({
			auth_date: "123",
			hash: "secret",
			user: JSON.stringify({ id: 42, first_name: "Ada" }),
		}).toString();
		expect(telegramUserScope(initData)).toBe("telegram-user:42");
		expect(
			scopedUserQueryKey(["favorites"], telegramUserScope(initData)),
		).toEqual(["favorites", { userScope: "telegram-user:42" }]);
	});

	it("uses a non-secret fallback for malformed data", () => {
		expect(telegramUserScope("user=not-json&hash=secret")).toBe(
			"telegram-user:unknown",
		);
	});
});
