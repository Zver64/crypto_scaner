import { describe, expect, it } from "vitest";
import { historyLoadDepth } from "@/features/command-settings/constants";
import { historyLoadRequest } from "@/features/command-settings/utils";

describe("history load request", () => {
	const values = {
		coins: ["BTCUSDT"],
		depth: 2001,
		intervals: ["1h"],
	};

	it("builds the request from complete values", () => {
		expect(historyLoadRequest(values)).toEqual({
			depth: 2001,
			intervals: ["1h"],
			symbols: ["BTCUSDT"],
		});
	});

	it("sorts the intervals into chart order", () => {
		expect(
			historyLoadRequest({ ...values, intervals: ["1M", "1h", "1d"] })
				?.intervals,
		).toEqual(["1h", "1d", "1M"]);
	});

	it("rejects missing choices and depths outside the limits", () => {
		expect(historyLoadRequest({ ...values, coins: [] })).toBeUndefined();
		expect(historyLoadRequest({ ...values, intervals: [] })).toBeUndefined();
		expect(
			historyLoadRequest({ ...values, intervals: ["unknown"] }),
		).toBeUndefined();
		expect(historyLoadRequest({ ...values, depth: "" })).toBeUndefined();
		expect(
			historyLoadRequest({ ...values, depth: historyLoadDepth.min - 1 }),
		).toBeUndefined();
		expect(
			historyLoadRequest({ ...values, depth: historyLoadDepth.max + 1 }),
		).toBeUndefined();
	});
});
