import { expect, it } from "vitest";
import { formatMarketCapUsd } from "@/utils/market-cap";

it("formats compact USD values", () => {
	expect(formatMarketCapUsd(1_234_567)).toBe("$1.2M");
});
