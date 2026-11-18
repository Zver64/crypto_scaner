import { describe, expect, it } from "vitest";

import type { APIErrorCode } from "@/api/generated/models";
import {
	type ApiError,
	describeApiError,
	describeQueryError,
} from "@/utils/api-error";

function failure(code: APIErrorCode, status?: number): ApiError {
	return Object.assign(new Error("failed"), {
		info: { error: { code, message: `server ${code}` }, request_id: "id" },
		status,
	});
}

const options = {
	fallback: "fallback",
	forbidden: "forbidden",
	messages: { strategy_exists: "exists" },
	server: ["invalid_argument"],
} as const;

describe("describeApiError", () => {
	it.each([
		["invalid_argument", "server invalid_argument"],
		["strategy_exists", "exists"],
		["access_denied", "forbidden"],
		["administrator_required", "forbidden"],
		[
			"unauthenticated",
			"Telegram authorization has expired. Reopen the Mini App.",
		],
		["internal_error", "fallback"],
	] as const)("describes %s", (code, expected) => {
		expect(describeApiError(failure(code), options)).toBe(expected);
	});

	it("falls back for access codes without a forbidden message", () => {
		expect(
			describeApiError(failure("access_denied"), { fallback: "fallback" }),
		).toBe("fallback");
	});

	it("falls back without a response body", () => {
		expect(describeApiError(new Error("offline"), options)).toBe("fallback");
	});
});

describe("describeQueryError", () => {
	it.each([
		[
			failure("access_denied"),
			"Your Telegram account does not have access to this scanner.",
		],
		[
			failure("unauthenticated"),
			"Your Telegram authorization has expired. Reopen the Mini App and try again.",
		],
		[
			failure("market_cap_missing", 422),
			"Market capitalization is unavailable for this instrument or is still loading. Try again shortly.",
		],
		[
			failure("administrator_required"),
			"An unexpected error occurred. Please try again.",
		],
		[new Error("offline"), "An unexpected error occurred. Please try again."],
	])("describes %#", (error, expected) => {
		expect(describeQueryError(error)).toBe(expected);
	});
});
