import { describe, expect, it } from "vitest";
import { indicatorScale, scaleDraft } from "@/features/scanner-settings/utils";

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
