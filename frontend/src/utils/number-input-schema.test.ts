import { describe, expect, it } from "vitest";

import { numberInputSchema } from "@/utils/number-input-schema";

describe("numberInputSchema", () => {
	const schema = numberInputSchema("Period", (number) => number.min(1));

	it.each([
		[3, 3],
		["1.50", 1.5],
		["2.0", 2],
		["1.", 1],
		["12345678901234567890", 12_345_678_901_234_567_000],
	])("parses %j to %s", (input, output) => {
		expect(schema.parse(input)).toBe(output);
	});

	it.each([
		["", "Period is required"],
		["-", "Period must be a number"],
		["abc", "Period must be a number"],
		[" ", "Period must be a number"],
	])("rejects %j", (input, message) => {
		expect(schema.safeParse(input).error?.issues[0]?.message).toBe(message);
	});

	it("applies the checks to a typed number", () => {
		expect(schema.safeParse("0.50").success).toBe(false);
	});
});
