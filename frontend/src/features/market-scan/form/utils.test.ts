import { expect, it } from "vitest";
import { defaultMarketScanCriteria } from "@/features/market-scan/pipeline";
import { criteriaFromValidDraft } from "./utils";

it("rejects incomplete and invalid numeric drafts", () => {
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: "30" }),
	).toBeUndefined();
	expect(
		criteriaFromValidDraft({ ...defaultMarketScanCriteria, period: 999_999 }),
	).toBeUndefined();
});
