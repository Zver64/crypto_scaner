import { expect, it } from "vitest";
import { criteriaFromValidDraft } from "@/features/market-scan/form/utils";
import { defaultMarketScanCriteria } from "@/features/market-scan/pipeline";

it("rejects incomplete and invalid numeric drafts", () => {
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: "30" }),
	).toBeUndefined();
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: 999_999 }),
	).toBeUndefined();
});
