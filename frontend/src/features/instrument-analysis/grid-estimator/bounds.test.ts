import { describe, expect, it } from "vitest";

import {
	gridBounds,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	markupScaleLabels,
	upperPriceFromMarkup,
	upperPriceLimitError,
} from "@/features/instrument-analysis/grid-estimator/bounds";
import {
	LOWER_MARKUP_MAX_PERCENT,
	UPPER_MARKUP_MAX_PERCENT,
} from "@/features/instrument-analysis/grid-estimator/config";

describe("gridBounds with Binance limits", () => {
	const limits = {
		askMultiplierUp: 1.5,
		averagePrice: 100,
		bidMultiplierDown: 0.25,
		maxPrice: 140,
		minPrice: 0.01,
		tickSize: 0.5,
	};
	const bounds = gridBounds("spot", undefined, limits);

	it("limits spot prices to the filter share and the symbol maximum", () => {
		// 100 × (1 − 0.75 × 0.85) = 36.25; min(140, 100 × (1 + 0.5 × 0.85)).
		expect(bounds.minPrice?.toString()).toBe("36.25");
		expect(bounds.maxPrice?.toString()).toBe("140");
		expect(bounds).toMatchObject({
			anchor: 100,
			lowerMarkupMax: 63.75,
			upperMarkupMax: 40,
		});
	});

	it("rounds markup maximums down and caps them", () => {
		expect(
			gridBounds("spot", undefined, { ...limits, bidMultiplierDown: 0.333 })
				.lowerMarkupMax,
		).toBe(56.69);
		expect(
			gridBounds("spot", undefined, {
				...limits,
				askMultiplierUp: 5,
				maxPrice: 0,
			}).upperMarkupMax,
		).toBe(200);
		expect(
			gridBounds("spot", undefined, {
				...limits,
				bidMultiplierDown: 0,
				minPrice: 0,
			}).lowerMarkupMax,
		).toBe(50);
	});

	it("rounds prices to the tick size and keeps them inside the limits", () => {
		expect(upperPriceFromMarkup(bounds, 5.3)).toBe("105.5");
		expect(lowerPriceFromMarkup(bounds, 10.3)).toBe("89.5");
		expect(upperPriceFromMarkup(bounds, 50)).toBe("140");
		expect(lowerPriceFromMarkup(bounds, 63.75)).toBe("36.5");
	});

	it("explains prices outside the limits", () => {
		expect(lowerPriceLimitError(bounds, "36", "USDT")).toBe(
			"Binance minimum is 36.5 USDT",
		);
		expect(lowerPriceLimitError(bounds, "36.25", "USDT")).toBeNull();
		expect(upperPriceLimitError(bounds, "140.5", "USDT")).toBe(
			"Binance maximum is 140 USDT",
		);
		expect(upperPriceLimitError(bounds, "140", "USDT")).toBeNull();
		expect(upperPriceLimitError(bounds, "", "USDT")).toBeNull();
	});

	it.each(["usdm", "coinm"] as const)(
		"does not apply the spot limits to %s grids",
		(market) => {
			const futures = gridBounds(market, undefined, limits);

			expect(futures).toEqual({
				anchor: 100,
				lowerMarkupMax: LOWER_MARKUP_MAX_PERCENT,
				maxPrice: null,
				minPrice: null,
				tickSize: null,
				upperMarkupMax: UPPER_MARKUP_MAX_PERCENT,
			});
			expect(upperPriceLimitError(futures, "1000", "USDT")).toBeNull();
			expect(lowerPriceFromMarkup(futures, 10.3)).toBe("89.7");
		},
	);
});

describe("markupScaleLabels", () => {
	it("labels the default markup only inside the slider", () => {
		expect(markupScaleLabels(50, 5)).toEqual([
			{ label: "0%", position: 0 },
			{ label: "5%", position: 10 },
			{ label: "50%", position: 100 },
		]);
		expect(markupScaleLabels(4, 10)).toEqual([
			{ label: "0%", position: 0 },
			{ label: "4%", position: 100 },
		]);
	});
});
