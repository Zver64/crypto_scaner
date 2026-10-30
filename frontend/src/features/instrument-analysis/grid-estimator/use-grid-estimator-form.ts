import { useSelector } from "@tanstack/react-form";
import { useState } from "react";

import { useAppForm } from "@/components/form";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import type {
	GridEstimatorFormValues,
	GridInputField,
	GridMarket,
	GridRangeValues,
	SpotGridLimits,
} from "@/features/instrument-analysis/grid-estimator/types";
import {
	calculateSpotGridInput,
	defaultInvestment,
	gridBounds,
	gridCountForRange,
	gridMarketEstimate,
	gridPriceLevels,
	gridRecommendation,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	priceAsset,
	spotGridMinimumStepPercent,
	upperMarkupPercent,
	upperPriceFromMarkup,
	upperPriceLimitError,
	validPositiveNumber,
} from "@/features/instrument-analysis/grid-estimator/utils";
import type { GridInput, GridType } from "@/utils/calculator/types";
import { formatRangePercent } from "@/utils/range-percent";

export interface GridEstimatorFormOptions {
	baseAsset: string;
	candles?: readonly (PriceCandle | null)[];
	dailyVolatilityPercent?: number;
	gridLimits?: SpotGridLimits | null;
	hourlyVolatilityPercent?: number;
	market: GridMarket;
	// Called with the levels of the committed grid when the form mounts and
	// whenever they are committed.
	onPriceLevelsChange(levels: readonly number[] | null): void;
}

/**
 * Keeps a grid calculator's form: the typed values, the committed input the
 * estimate is calculated from, and how sliders and typed prices update them.
 */
export function useGridEstimatorForm({
	baseAsset,
	candles,
	dailyVolatilityPercent,
	gridLimits,
	hourlyVolatilityPercent,
	market,
	onPriceLevelsChange,
}: GridEstimatorFormOptions) {
	// The parent remounts the calculator once its data is ready, so the price
	// both markups are measured from stays fixed while it is being edited.
	const [bounds] = useState(() => gridBounds(market, candles, gridLimits));
	const [recommendation] = useState(() =>
		gridRecommendation(
			bounds,
			hourlyVolatilityPercent,
			"geometric",
			defaultInvestment(market, bounds.anchor),
		),
	);
	const [defaultValues] = useState((): GridEstimatorFormValues => ({
		...recommendation.input,
		direction: "short",
		gridType: "geometric",
		leverage: 1,
		lowerMarkup: recommendation.lowerMarkup,
		markup: recommendation.upperMarkup,
		rangePercent: hourlyVolatilityPercent ?? 0,
	}));
	const form = useAppForm({
		defaultValues,
		listeners: {
			onMount: () =>
				onPriceLevelsChange(
					gridPriceLevels(recommendation.input, defaultValues.gridType),
				),
		},
	});
	// The sliders, markups, futures options, and estimate follow the values as
	// they change.
	const values = useSelector(form.store, (state) => state.values);
	const [committedInput, setCommittedInput] = useState<GridInput>(
		recommendation.input,
	);
	// Commits an input and reports the levels when the range changes. applyRange
	// passes the grid type it has just set; other commits keep the current one.
	const commitInput = (
		input: GridInput,
		gridType: GridType = values.gridType,
	) => {
		setCommittedInput(input);
		if (
			input.lowerPrice !== committedInput.lowerPrice ||
			input.upperPrice !== committedInput.upperPrice ||
			input.gridCount !== committedInput.gridCount ||
			gridType !== values.gridType
		) {
			onPriceLevelsChange(gridPriceLevels(input, gridType));
		}
	};
	// Sets a typed field's error the way its blur validator does, so the next
	// blur checks the field again.
	const setBlurError = (field: GridInputField, error: string | undefined) =>
		form.setFieldMeta(field, (meta) => ({
			...meta,
			errorMap: { ...meta.errorMap, onBlur: error },
		}));

	const { anchor } = bounds;
	const hourlyRange = validPositiveNumber(hourlyVolatilityPercent)
		? hourlyVolatilityPercent
		: null;
	const dailyRange = validPositiveNumber(dailyVolatilityPercent)
		? dailyVolatilityPercent
		: null;
	const priceUnit = priceAsset(market);
	const { direction, gridType, leverage } = values;
	const estimate = gridMarketEstimate(
		market,
		committedInput,
		{ currentPrice: anchor, direction, gridType, leverage },
		baseAsset,
	);

	// Applies a new price range, step, or grid type and derives the largest grid
	// count whose minimum step stays at or above the selected step.
	function applyRange(
		range: GridRangeValues,
		markups: Partial<
			Pick<GridEstimatorFormValues, "lowerMarkup" | "markup">
		> = {},
	) {
		const { error, gridCount } = gridCountForRange(
			range,
			committedInput.gridCount,
			anchor,
			markups.lowerMarkup ?? values.lowerMarkup,
		);
		form.setFieldValue("gridType", range.gridType);
		form.setFieldValue("lowerPrice", range.lowerPrice);
		form.setFieldValue("rangePercent", range.rangePercent);
		form.setFieldValue("upperPrice", range.upperPrice);
		form.setFieldValue("gridCount", gridCount);
		if (markups.lowerMarkup !== undefined) {
			form.setFieldValue("lowerMarkup", markups.lowerMarkup);
		}
		if (markups.markup !== undefined) {
			form.setFieldValue("markup", markups.markup);
		}
		setBlurError("gridCount", error ?? undefined);
		setBlurError("lowerPrice", undefined);
		setBlurError("upperPrice", undefined);
		commitInput(
			{
				...committedInput,
				gridCount,
				lowerPrice: range.lowerPrice,
				upperPrice: range.upperPrice,
			},
			range.gridType,
		);
	}

	function currentRange(): GridRangeValues {
		return {
			gridType: values.gridType,
			lowerPrice: committedInput.lowerPrice,
			rangePercent: values.rangePercent,
			upperPrice: committedInput.upperPrice,
		};
	}

	function changeMarkup(markup: number) {
		const upperPrice = upperPriceFromMarkup(bounds, markup);
		if (!upperPrice) return;
		applyRange({ ...currentRange(), upperPrice }, { markup });
	}

	function changeLowerMarkup(lowerMarkup: number) {
		const lowerPrice = lowerPriceFromMarkup(bounds, lowerMarkup);
		if (!lowerPrice) {
			form.setFieldValue("lowerMarkup", lowerMarkup);
			return;
		}
		applyRange({ ...currentRange(), lowerPrice }, { lowerMarkup });
	}

	function changeRange(rangePercent: number) {
		applyRange({ ...currentRange(), rangePercent });
	}

	function changeGridType(gridType: GridType) {
		applyRange({ ...currentRange(), gridType });
	}

	function commitUpperPrice(upperPrice: string) {
		const markup = upperMarkupPercent(anchor, upperPrice);
		if (markup === null) {
			commitInput({ ...committedInput, upperPrice });
			return;
		}
		applyRange({ ...currentRange(), upperPrice }, { markup });
	}

	function commitLowerPrice(lowerPrice: string) {
		const lowerMarkup = lowerMarkupPercent(anchor, lowerPrice);
		if (lowerMarkup === null) {
			commitInput({ ...committedInput, lowerPrice });
			return;
		}
		applyRange({ ...currentRange(), lowerPrice }, { lowerMarkup });
	}

	// The minimum step of a typed grid count, when the hourly range bounds
	// it. Spot and futures grids share their levels, so the spot estimate
	// gives the step.
	function gridCountStepPercent(gridCount: string): number | null {
		const spotEstimate = calculateSpotGridInput(
			{ ...committedInput, gridCount },
			values.gridType,
		)?.estimate;
		return spotEstimate && hourlyRange !== null
			? spotGridMinimumStepPercent(spotEstimate)
			: null;
	}

	// Accepts a typed grid count only when its minimum step stays within the
	// hourly-to-daily range.
	function gridCountError(gridCount: string): string | undefined {
		const stepPercent = gridCountStepPercent(gridCount);
		if (
			stepPercent === null ||
			hourlyRange === null ||
			(stepPercent >= hourlyRange &&
				(dailyRange === null || stepPercent <= dailyRange))
		) {
			return undefined;
		}
		return `Minimum grid step would be ${formatRangePercent(stepPercent)}; it must be ${
			dailyRange === null
				? `at least ${formatRangePercent(hourlyRange)}`
				: `from ${formatRangePercent(hourlyRange)} to ${formatRangePercent(dailyRange)}`
		}`;
	}

	// Commits an accepted grid count and moves the step slider to its step.
	function commitGridCount(gridCount: string) {
		const stepPercent = gridCountStepPercent(gridCount);
		if (stepPercent !== null) form.setFieldValue("rangePercent", stepPercent);
		commitInput({ ...committedInput, gridCount });
	}

	function commitField(field: GridInputField) {
		const value = form.getFieldValue(field);
		if (value === committedInput[field]) return;

		if (field === "upperPrice") commitUpperPrice(value);
		else if (field === "lowerPrice") commitLowerPrice(value);
		else if (field === "gridCount") commitGridCount(value);
		else commitInput({ ...committedInput, [field]: value });
	}

	function applyInvestment(investment: string) {
		form.setFieldValue("investment", investment);
		commitInput({ ...committedInput, investment });
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
		if (field === "gridCount") return gridCountError(value);
		return undefined;
	}

	return {
		applyInvestment,
		bounds,
		changeGridType,
		changeLowerMarkup,
		changeMarkup,
		changeRange,
		dailyRange,
		estimate,
		commitError,
		commitField,
		committedInput,
		form,
		hourlyRange,
		priceUnit,
		values,
	};
}
