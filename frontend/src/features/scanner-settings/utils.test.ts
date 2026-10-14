import { describe, expect, it } from "vitest";

import type { IndicatorType } from "@/api/generated/models";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";
import {
	defaultParameterValues,
	indicatorScale,
	scaleDraft,
	scannerIndicatorInput,
} from "@/features/scanner-settings/utils";

describe("indicator scale", () => {
	it("round-trips a saved scale through the editor draft", () => {
		const scale = { levels: [30, 70], max: 100, min: 0 };
		expect(indicatorScale(scaleDraft(scale))).toEqual(scale);
	});

	it("leaves empty bounds automatic", () => {
		expect(indicatorScale(scaleDraft({ levels: [] }))).toEqual({ levels: [] });
	});

	it("rejects levels that are not numbers", () => {
		expect(
			indicatorScale({ levels: ["30", "high"], scaleMax: "", scaleMin: "" }),
		).toBeUndefined();
	});
});

describe("scanner indicator input", () => {
	const type: IndicatorType = {
		group: "Momentum",
		outputs: ["rsi"],
		overlay: false,
		parameters: [
			{
				choices: [
					{ title: "Close", value: 0 },
					{ title: "Open", value: 1 },
				],
				default: 0,
				description: "",
				key: "source",
				kind: "choice",
				title: "Source",
			},
			{
				default: 14,
				description: "",
				key: "period",
				kind: "integer",
				title: "Period",
			},
		],
		title: "Relative Strength Index",
		type: "rsi",
	};
	const draft: ScannerIndicatorDraft = {
		levels: ["30"],
		parameters: { ...defaultParameterValues(type), source: "1" },
		periods: { "1d": { showInChart: true, showInTable: true } },
		scaleMax: "",
		scaleMin: "",
		type: "rsi",
	};

	it("converts the inputs to the request", () => {
		expect(scannerIndicatorInput(draft, type, ["1h", "1d"])).toEqual({
			intervals: [{ interval: "1d", show_in_chart: true, show_in_table: true }],
			parameters: { period: 14, source: 1 },
			scale: { levels: [30] },
			type: "rsi",
		});
	});

	it("is undefined without a type, a period, or valid levels", () => {
		expect(scannerIndicatorInput(draft, undefined, ["1d"])).toBeUndefined();
		expect(scannerIndicatorInput(draft, type, ["1h"])).toBeUndefined();
		expect(
			scannerIndicatorInput({ ...draft, levels: ["x"] }, type, ["1d"]),
		).toBeUndefined();
	});
});
