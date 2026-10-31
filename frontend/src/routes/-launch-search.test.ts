import { expect, it } from "vitest";

import { parseLaunchSearch } from "@/routes/-launch-search";

it("keeps the coin of a launch link and drops other start parameters", () => {
	expect(parseLaunchSearch({ tgWebAppStartParam: "BTCUSDT" })).toEqual({
		tgWebAppStartParam: "BTCUSDT",
	});
	expect(parseLaunchSearch({ tgWebAppStartParam: "promo-1" })).toEqual({});
});
