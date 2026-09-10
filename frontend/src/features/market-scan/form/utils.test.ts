import { expect, it } from "vitest";
import { defaultMarketScanCriteria } from "@/features/market-scan/pipeline";
import { criteriaFromValidDraft } from "./utils";

it("converts only complete numeric drafts to criteria", () => {
	expect(criteriaFromValidDraft(defaultMarketScanCriteria)).toEqual(
		defaultMarketScanCriteria,
	);
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: "30" }),
	).toBeUndefined();
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: 999_999 }),
	).toBeUndefined();
});
