import { expect, it } from "vitest";
import { defaultMarketScanCriteria } from "@/features/market-scan/pipeline";
import { criteriaAreEqual, criteriaFromValidDraft } from "./utils";

it("compares all Market Scan criteria", () => {
	expect(
		criteriaAreEqual(defaultMarketScanCriteria, {
			...defaultMarketScanCriteria,
		}),
	).toBe(true);
	expect(
		criteriaAreEqual(defaultMarketScanCriteria, {
			...defaultMarketScanCriteria,
			hourlyPeriod: 24,
		}),
	).toBe(false);
});

it("converts only complete numeric drafts to criteria", () => {
	expect(criteriaFromValidDraft(defaultMarketScanCriteria)).toEqual(
		defaultMarketScanCriteria,
	);
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: "30" }),
	).toBeUndefined();
});
