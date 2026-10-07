import { describe, expect, it } from "vitest";
import type { ErrorType } from "@/api/fetch";
import type { APIErrorCode, ErrorResponse } from "@/api/generated/models";
import { describeApiError } from "@/utils/api-error";

function failure(code: APIErrorCode): ErrorType<ErrorResponse> {
	return Object.assign(new Error("failed"), {
		info: { error: { code, message: `server ${code}` }, request_id: "id" },
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
