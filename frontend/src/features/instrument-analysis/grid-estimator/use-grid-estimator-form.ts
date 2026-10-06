import { useForm } from "@mantine/form";
import { type FocusEvent, type KeyboardEvent, useMemo, useState } from "react";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import { DEFAULT_MARKUPS } from "@/features/instrument-analysis/grid-estimator/config";
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
}: GridEstimatorFormOptions) {
	// The parent remounts the calculator once its data is ready, so the price
	// both markups are measured from stays fixed while it is being edited.
	const [bounds] = useState(() => gridBounds(market, candles, gridLimits));
	const [recommendation] = useState(() =>
		gridRecommendation(
			bounds,
			hourlyVolatilityPercent,
			"geometric",
			DEFAULT_MARKUPS[market],
			defaultInvestment(market, bounds.anchor),
		),
	);
	const form = useForm<GridEstimatorFormValues>({
		initialValues: {
			...recommendation.input,
			direction: "short",
			gridType: "geometric",
			leverage: 1,
			lowerMarkup: recommendation.lowerMarkup,
			markup: recommendation.upperMarkup,
			rangePercent: hourlyVolatilityPercent ?? 0,
		},
		mode: "controlled",
	});
	const [committedInput, setCommittedInput] = useState<GridInput>(
		recommendation.input,
	);

	const { anchor } = bounds;
	const hourlyRange = validPositiveNumber(hourlyVolatilityPercent)
		? hourlyVolatilityPercent
		: null;
	const dailyRange = validPositiveNumber(dailyVolatilityPercent)
		? dailyVolatilityPercent
		: null;
	const priceUnit = priceAsset(market);
	const { direction, gridType, leverage } = form.values;
	const estimate = useMemo(
		() =>
			gridMarketEstimate(
				market,
				committedInput,
				{ currentPrice: anchor, direction, gridType, leverage },
				baseAsset,
			),
		[anchor, baseAsset, committedInput, direction, gridType, leverage, market],
	);

	// Applies a new price range, step, or grid type and derives the largest grid
	// count whose minimum step stays at or above the selected step.
	function applyRange(
		range: GridRangeValues,
		extraValues: Partial<GridEstimatorFormValues> = {},
	) {
		const { error, gridCount } = gridCountForRange(
			range,
			committedInput.gridCount,
			anchor,
			extraValues.lowerMarkup ?? form.values.lowerMarkup,
		);
		form.setValues({ ...range, ...extraValues, gridCount });
		form.clearFieldError("gridCount");
		form.clearFieldError("lowerPrice");
		form.clearFieldError("upperPrice");
		if (error) form.setFieldError("gridCount", error);
		setCommittedInput({
			...committedInput,
			gridCount,
			lowerPrice: range.lowerPrice,
			upperPrice: range.upperPrice,
		});
	}

	function currentRange(): GridRangeValues {
		return {
			gridType: form.values.gridType,
			lowerPrice: committedInput.lowerPrice,
			rangePercent: form.values.rangePercent,
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
		const limitError = upperPriceLimitError(bounds, upperPrice, priceUnit);
		if (limitError) {
			form.setFieldError("upperPrice", limitError);
			return;
		}
		const markup = upperMarkupPercent(anchor, upperPrice);
		if (markup === null) {
			setCommittedInput({ ...committedInput, upperPrice });
			return;
		}
		applyRange({ ...currentRange(), upperPrice }, { markup });
	}

	function commitLowerPrice(lowerPrice: string) {
		const limitError = lowerPriceLimitError(bounds, lowerPrice, priceUnit);
		if (limitError) {
			form.setFieldError("lowerPrice", limitError);
			return;
		}
		const lowerMarkup = lowerMarkupPercent(anchor, lowerPrice);
		if (lowerMarkup === null) {
			setCommittedInput({ ...committedInput, lowerPrice });
			return;
		}
		applyRange({ ...currentRange(), lowerPrice }, { lowerMarkup });
	}

	// Accepts a typed grid count only when its minimum step stays within the
	// hourly-to-daily range, then moves the step slider to that step. Spot and
	// futures grids share their levels, so the spot estimate gives the step.
	function commitGridCount(gridCount: string) {
		const nextInput = { ...committedInput, gridCount };
		const spotEstimate = calculateSpotGridInput(
			nextInput,
			form.values.gridType,
		)?.estimate;
		if (!spotEstimate || hourlyRange === null) {
			setCommittedInput(nextInput);
			return;
		}

		const stepPercent = spotGridMinimumStepPercent(spotEstimate);
		if (
			stepPercent < hourlyRange ||
			(dailyRange !== null && stepPercent > dailyRange)
		) {
			form.setFieldError(
				"gridCount",
				`Minimum grid step would be ${formatRangePercent(stepPercent)}; it must be ${
					dailyRange === null
						? `at least ${formatRangePercent(hourlyRange)}`
						: `from ${formatRangePercent(hourlyRange)} to ${formatRangePercent(dailyRange)}`
				}`,
			);
			return;
		}
		form.setFieldValue("rangePercent", stepPercent);
		setCommittedInput(nextInput);
	}

	function commitField(field: GridInputField) {
		const value = form.getValues()[field];
		if (value === committedInput[field]) return;

		if (field === "upperPrice") commitUpperPrice(value);
		else if (field === "lowerPrice") commitLowerPrice(value);
		else if (field === "gridCount") commitGridCount(value);
		else setCommittedInput({ ...committedInput, [field]: value });
	}

	function applyInvestment(investment: string) {
		form.setFieldValue("investment", investment);
		form.clearFieldError("investment");
		setCommittedInput({ ...committedInput, investment });
	}

	// Commits a typed field on blur or Enter and restores it on Escape.
	function inputProps(field: GridInputField) {
		const props = form.getInputProps(field);
		return {
			...props,
			onBlur: (event: FocusEvent<HTMLInputElement>) => {
				props.onBlur(event);
				commitField(field);
			},
			onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => {
				if (event.key === "Enter") {
					event.preventDefault();
					commitField(field);
				}
				if (event.key === "Escape") {
					event.preventDefault();
					form.setFieldValue(field, committedInput[field]);
					form.clearFieldError(field);
				}
			},
		};
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
		form,
		hourlyRange,
		inputProps,
		priceUnit,
	};
}
