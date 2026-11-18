import { useSelector } from "@tanstack/react-form";

import type { GridLimitsResponse } from "@/api/generated/models";
import {
	gridBounds,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	upperMarkupPercent,
	upperPriceFromMarkup,
	upperPriceLimitError,
	validPositiveNumber,
} from "@/features/instrument-analysis/grid-estimator/bounds";
import type { GridInput } from "@/features/instrument-analysis/grid-estimator/calculator/types";
import {
	gridCountRange,
	gridMarketEstimate,
	priceAsset,
} from "@/features/instrument-analysis/grid-estimator/estimate";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import {
	evenLowerBound,
	evenUpperBound,
	gridCountCap,
	gridCountForRange,
	gridCountStepError,
	gridCountStepPercent,
} from "@/features/instrument-analysis/grid-estimator/grid-range";
import { marketMinimumInvestment } from "@/features/instrument-analysis/grid-estimator/investment-minimum";
import type {
	GridCountCap,
	GridInputField,
	GridMarket,
	GridMarketFormValues,
	GridRangeValues,
} from "@/features/instrument-analysis/grid-estimator/types";
import type { useGridCalculatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-calculator-form";

export interface GridMarketOptions {
	baseAsset: string;
	dailyVolatilityPercent?: number;
	form: ReturnType<typeof useGridCalculatorForm>;
	gridLimits: GridLimitsResponse | null;
	hourlyVolatilityPercent?: number;
	market: GridMarket;
}

// The range of a market's values: the committed prices with the selected step
// and grid type.
function committedRange(values: GridMarketFormValues): GridRangeValues {
	return {
		gridType: values.gridType,
		lowerPrice: values.committed.lowerPrice,
		rangePercent: values.rangePercent,
		upperPrice: values.committed.upperPrice,
	};
}

/**
 * One market of the grid calculator: its values, the estimate of its committed
 * input, and how sliders and typed prices update them. Handlers read the form
 * when they run, after the field that called them has changed.
 */
export function useGridMarket({
	baseAsset,
	dailyVolatilityPercent,
	form,
	gridLimits,
	hourlyVolatilityPercent,
	market,
}: GridMarketOptions) {
	const values = useSelector(
		form.store,
		(state) => state.values.markets[market],
	);
	const defaultRangePercent = useSelector(
		form.store,
		(state) => state.values.defaultRangePercent,
	);
	// The bounds of the close the market was built from, so the price both
	// markups are measured from stays fixed while it is being edited.
	const bounds = gridBounds(market, values.close, gridLimits?.spot);
	const { anchor } = bounds;
	const hourlyRange = validPositiveNumber(hourlyVolatilityPercent)
		? hourlyVolatilityPercent
		: null;
	const dailyRange = validPositiveNumber(dailyVolatilityPercent)
		? dailyVolatilityPercent
		: null;
	const priceUnit = priceAsset(market);
	const { committed, direction, gridType, leverage } = values;
	const estimate = gridMarketEstimate(
		market,
		committed,
		{ currentPrice: anchor, direction, gridType, leverage },
		baseAsset,
	);
	// The smallest investment Binance accepts for the committed grid; the
	// form checks the investment against it.
	const minimumInvestment =
		marketMinimumInvestment(market, values, gridLimits, baseAsset)?.text ??
		null;
	// The grid counts Binance accepts for the committed grid; the form checks
	// the grid count against them.
	const gridCounts = gridCountRange(market, committed, gridType, gridLimits);
	const maxCount: GridCountCap = (range) =>
		gridCountCap(market, range, gridLimits);

	const current = () => form.state.values.markets[market];
	// Values are written as their inputs would, without running the listeners
	// that call these handlers.
	const write = { dontRunListeners: true };
	const commitInput = (input: GridInput) =>
		form.setFieldValue(marketField(market, "committed"), input, write);
	// Sets a typed field's error the way its blur validator does, so the next
	// blur checks the field again.
	const setBlurError = (field: GridInputField, error: string | undefined) =>
		form.setFieldMeta(marketField(market, field), (meta) => ({
			...meta,
			errorMap: { ...meta.errorMap, onBlur: error },
		}));

	// Applies a new price range, step, or grid type and derives the largest grid
	// count whose minimum step stays at or above the selected step.
	function applyRange(
		range: GridRangeValues,
		markups: Partial<Pick<GridMarketFormValues, "lowerMarkup" | "markup">> = {},
	) {
		const { committed: input, lowerMarkup } = current();
		const { gridCount } = gridCountForRange(
			range,
			input.gridCount,
			maxCount(range),
			anchor,
			markups.lowerMarkup ?? lowerMarkup,
		);
		form.setFieldValue(marketField(market, "gridType"), range.gridType, write);
		form.setFieldValue(
			marketField(market, "lowerPrice"),
			range.lowerPrice,
			write,
		);
		form.setFieldValue(
			marketField(market, "rangePercent"),
			range.rangePercent,
			write,
		);
		form.setFieldValue(
			marketField(market, "upperPrice"),
			range.upperPrice,
			write,
		);
		form.setFieldValue(marketField(market, "gridCount"), gridCount, write);
		if (markups.lowerMarkup !== undefined) {
			form.setFieldValue(
				marketField(market, "lowerMarkup"),
				markups.lowerMarkup,
				write,
			);
		}
		if (markups.markup !== undefined) {
			form.setFieldValue(marketField(market, "markup"), markups.markup, write);
		}
		setBlurError("gridCount", undefined);
		setBlurError("lowerPrice", undefined);
		setBlurError("upperPrice", undefined);
		commitInput({
			...input,
			gridCount,
			lowerPrice: range.lowerPrice,
			upperPrice: range.upperPrice,
		});
	}

	// Why the selected step does not fit the range, however it came about.
	const { error: rangeError } = gridCountForRange(
		committedRange(values),
		committed.gridCount,
		maxCount(committedRange(values)),
		anchor,
		values.lowerMarkup,
	);

	// Moves a range bound to where the grid is even; null when the range is
	// already even or the bound cannot move.
	const evenUpper = evenUpperBound(
		bounds,
		committedRange(values),
		values.lowerMarkup,
		maxCount,
	);
	const alignUpperPrice =
		evenUpper &&
		(() =>
			applyRange(
				{ ...committedRange(values), upperPrice: evenUpper.upperPrice },
				{ markup: evenUpper.markup },
			));
	const evenLower = evenLowerBound(
		bounds,
		committedRange(values),
		values.lowerMarkup,
		maxCount,
	);
	const alignLowerPrice =
		evenLower &&
		(() =>
			applyRange(
				{ ...committedRange(values), lowerPrice: evenLower.lowerPrice },
				{ lowerMarkup: evenLower.lowerMarkup },
			));

	// Listeners of the markup, step, and grid type inputs, which have already
	// set their value.
	function changeMarkup(markup: number) {
		const upperPrice = upperPriceFromMarkup(bounds, markup);
		if (!upperPrice) return;
		applyRange({ ...committedRange(current()), upperPrice }, { markup });
	}

	function changeLowerMarkup(lowerMarkup: number) {
		const lowerPrice = lowerPriceFromMarkup(bounds, lowerMarkup);
		if (!lowerPrice) return;
		applyRange({ ...committedRange(current()), lowerPrice }, { lowerMarkup });
	}

	function changeRange() {
		applyRange(committedRange(current()));
	}

	function commitUpperPrice(upperPrice: string) {
		const markup = upperMarkupPercent(anchor, upperPrice);
		if (markup === null) {
			commitInput({ ...current().committed, upperPrice });
			return;
		}
		applyRange({ ...committedRange(current()), upperPrice }, { markup });
	}

	function commitLowerPrice(lowerPrice: string) {
		const lowerMarkup = lowerMarkupPercent(anchor, lowerPrice);
		if (lowerMarkup === null) {
			commitInput({ ...current().committed, lowerPrice });
			return;
		}
		applyRange({ ...committedRange(current()), lowerPrice }, { lowerMarkup });
	}

	const typedCountStep = (gridCount: string) => {
		const marketValues = current();
		return gridCountStepPercent(
			{ ...marketValues.committed, gridCount },
			marketValues.gridType,
		);
	};

	// Commits an accepted grid count and moves the step slider to its step.
	function commitGridCount(gridCount: string) {
		const stepPercent = typedCountStep(gridCount);
		if (stepPercent !== null && hourlyRange !== null) {
			form.setFieldValue(
				marketField(market, "rangePercent"),
				stepPercent,
				write,
			);
		}
		commitInput({ ...current().committed, gridCount });
	}

	// Listener of a typed field's blur once its value is accepted.
	function commitField(field: GridInputField) {
		const marketValues = current();
		const value = marketValues[field];
		if (value === marketValues.committed[field]) return;

		if (field === "upperPrice") commitUpperPrice(value);
		else if (field === "lowerPrice") commitLowerPrice(value);
		else if (field === "gridCount") commitGridCount(value);
		else commitInput({ ...marketValues.committed, [field]: value });
	}

	// Sets a preset investment as if it was typed: shown with its errors, and
	// committed, like a typed value on blur, only without them.
	function applyInvestment(investment: string) {
		const field = marketField(market, "investment");
		form.setFieldValue(field, investment, write);
		if (form.getFieldMeta(field)?.errors.length === 0)
			commitInput({ ...current().committed, investment });
	}

	// Why a typed field cannot be committed; checked on blur.
	function commitError(
		field: GridInputField,
		value: string,
	): string | undefined {
		if (field === "upperPrice") {
			return upperPriceLimitError(bounds, value, priceUnit) ?? undefined;
		}
		if (field === "lowerPrice") {
			return lowerPriceLimitError(bounds, value, priceUnit) ?? undefined;
		}
		if (field === "gridCount") {
			return (
				gridCountStepError(typedCountStep(value), hourlyRange, dailyRange) ??
				undefined
			);
		}
		return undefined;
	}

	return {
		alignLowerPrice,
		alignUpperPrice,
		applyInvestment,
		bounds,
		changeLowerMarkup,
		changeMarkup,
		changeRange,
		commitError,
		commitField,
		dailyRange,
		defaultRangePercent,
		estimate,
		gridCounts,
		hourlyRange,
		minimumInvestment,
		priceUnit,
		rangeError,
		values,
	};
}
