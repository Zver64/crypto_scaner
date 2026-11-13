import { describe, expect, it } from "vitest";

import type { IndicatorType, ScannerIndicator } from "@/api/generated/models";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";
import {
	defaultParameterValues,
	indicatorScale,
	scaleDraft,
	scannerIndicatorInput,
	scannerIndicatorSettingsDraft,
	scannerIndicatorUpdate,
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
		periods: {
			"1d": { showInChart: true, showInTable: true },
			"1h": { showInChart: false, showInTable: true },
		},
		scaleMax: "",
		scaleMin: "",
		type: "rsi",
	};

	it("converts the inputs to the request, periods in the given order", () => {
		expect(scannerIndicatorInput(draft, type, ["1h", "1d"])).toEqual({
			parameters: { period: 14, source: 1 },
			periods: [
				{ interval: "1h", show_in_chart: false, show_in_table: true },
				{ interval: "1d", show_in_chart: true, show_in_table: true },
			],
			scale: { levels: [30] },
			type: "rsi",
		});
	});

	it("allows an indicator without periods", () => {
		expect(
			scannerIndicatorInput({ ...draft, periods: {} }, type, ["1h", "1d"])
				?.periods,
		).toEqual([]);
	});

	it("is undefined without a type or valid levels", () => {
		expect(scannerIndicatorInput(draft, undefined, ["1d"])).toBeUndefined();
		expect(
			scannerIndicatorInput({ ...draft, levels: ["x"] }, type, ["1d"]),
		).toBeUndefined();
	});
});

describe("scanner indicator update", () => {
	const indicator: ScannerIndicator = {
		id: 1,
		outputs: ["upperband", "middleband", "lowerband"],
		parameters: { period: 20 },
		periods: [{ interval: "1d", show_in_chart: true, show_in_table: false }],
		placement: "overlay",
		scale: { levels: [] },
		title: "bbands",
		type: "bbands",
	};

	it("round-trips the saved periods and keeps several outputs out of tables", () => {
		const draft = scannerIndicatorSettingsDraft(indicator);
		expect(scannerIndicatorUpdate(draft, indicator, ["1h", "1d"])).toEqual({
			periods: indicator.periods,
		});
		expect(
			scannerIndicatorUpdate(
				{
					...draft,
					periods: { "1h": { showInChart: false, showInTable: true } },
				},
				indicator,
				["1h", "1d"],
			),
		).toEqual({
			periods: [{ interval: "1h", show_in_chart: false, show_in_table: false }],
		});
	});

	it("sends the scale of a pane indicator only", () => {
		const pane: ScannerIndicator = {
			...indicator,
			outputs: ["rsi"],
			placement: "pane",
			scale: { levels: [30, 70], max: 100, min: 0 },
		};
		expect(
			scannerIndicatorUpdate(scannerIndicatorSettingsDraft(pane), pane, ["1d"]),
		).toEqual({ periods: pane.periods, scale: pane.scale });
	});
});
