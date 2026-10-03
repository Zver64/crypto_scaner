import {
	Alert,
	Center,
	Paper,
	SegmentedControl,
	SimpleGrid,
	Stack,
	Text,
	TextInput,
	Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { type FocusEvent, type KeyboardEvent, useMemo, useState } from "react";
import { SegmentedValueGroup } from "@/components/segmented-value-group";
import { SliderField } from "@/components/slider-field";
import { ValueGroup } from "@/components/value-group";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import {
	calculateSpotGridInput,
	DEFAULT_MARKUP_PERCENT,
	formatMaxPrice,
	formatMinPrice,
	gridCountForStep,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	type SpotGridLimits,
	type SpotGridType,
	spotGridBounds,
	spotGridEstimateValues,
	spotGridMinimumStepPercent,
	spotGridRecommendation,
	upperMarkupPercent,
	upperPriceFromMarkup,
	upperPriceLimitError,
} from "@/features/instrument-analysis/spot-grid-estimator/utils";
import type { SpotGridInput } from "@/utils/calculator/spot-grid";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

interface SpotGridEstimatorProps {
	candles?: readonly (PriceCandle | null)[];
	dailyVolatilityPercent?: number;
	disabled?: boolean;
	hourlyVolatilityPercent?: number;
	paperPadding: string;
	// Binance grid limits: undefined while loading, null when unavailable.
	gridLimits?: SpotGridLimits | null;
}

type SpotGridFormValues = SpotGridInput & {
	gridType: SpotGridType;
	lowerMarkup: number;
	markup: number;
	rangePercent: number;
};

type InputField = keyof SpotGridInput;

type RangeValues = Pick<
	SpotGridFormValues,
	"gridType" | "lowerPrice" | "rangePercent" | "upperPrice"
>;

function formValues(
	input: SpotGridInput,
	upperMarkup: number,
	lowerMarkup: number,
	hourlyRangePercent: number | undefined,
): SpotGridFormValues {
	return {
		...input,
		gridType: "geometric",
		lowerMarkup,
		markup: upperMarkup,
		rangePercent: hourlyRangePercent ?? 0,
	};
}

export function SpotGridEstimator({
	candles,
	dailyVolatilityPercent,
	disabled = false,
	hourlyVolatilityPercent,
	paperPadding,
	gridLimits,
}: SpotGridEstimatorProps) {
	// The parent remounts the calculator once its data is ready, so the price
	// both markups are measured from stays fixed while it is being edited.
	const [bounds] = useState(() => spotGridBounds(candles, gridLimits));
	const [recommendation] = useState(() =>
		spotGridRecommendation(bounds, hourlyVolatilityPercent),
	);
	const form = useForm<SpotGridFormValues>({
		initialValues: formValues(
			recommendation.input,
			recommendation.upperMarkup,
			recommendation.lowerMarkup,
			hourlyVolatilityPercent,
		),
		mode: "controlled",
	});
	const [committedInput, setCommittedInput] = useState<SpotGridInput>(
		recommendation.input,
	);

	const { anchor } = bounds;
	const hasAnchor = anchor !== null;
	const minPrice = formatMinPrice(bounds);
	const maxPrice = formatMaxPrice(bounds);
	const hasHourlyVolatility =
		typeof hourlyVolatilityPercent === "number" &&
		Number.isFinite(hourlyVolatilityPercent) &&
		hourlyVolatilityPercent > 0;
	const hasDailyVolatility =
		typeof dailyVolatilityPercent === "number" &&
		Number.isFinite(dailyVolatilityPercent) &&
		dailyVolatilityPercent > 0;
	const selectedRangePercent = form.values.rangePercent;
	const hourlyRangeValue = hasHourlyVolatility ? hourlyVolatilityPercent : 0;
	const dailyRangeValue = hasDailyVolatility ? dailyVolatilityPercent : 0;
	const canSelectRange =
		hasHourlyVolatility &&
		hasDailyVolatility &&
		dailyRangeValue > hourlyRangeValue;
	const calculation = useMemo(
		() => calculateSpotGridInput(committedInput, form.values.gridType),
		[committedInput, form.values.gridType],
	);
	const values = spotGridEstimateValues(calculation?.estimate ?? null);

	// Applies a new price range, step, or grid type and derives the largest grid
	// count whose minimum step stays at or above the selected step.
	function applyRange(
		range: RangeValues,
		extraValues: Partial<SpotGridFormValues> = {},
	) {
		const gridCount =
			range.rangePercent > 0
				? (gridCountForStep(
						range.upperPrice,
						range.lowerPrice,
						range.rangePercent,
						range.gridType,
						anchor,
						extraValues.lowerMarkup ?? form.values.lowerMarkup,
					) ?? "")
				: committedInput.gridCount;
		form.setValues({ ...range, ...extraValues, gridCount });
		form.clearFieldError("gridCount");
		form.clearFieldError("lowerPrice");
		form.clearFieldError("upperPrice");
		setCommittedInput({
			...committedInput,
			gridCount,
			lowerPrice: range.lowerPrice,
			upperPrice: range.upperPrice,
		});
	}

	function currentRange(): RangeValues {
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

	function changeGridType(gridType: SpotGridType) {
		applyRange({ ...currentRange(), gridType });
	}

	function commitUpperPrice(upperPrice: string) {
		const limitError = upperPriceLimitError(bounds, upperPrice);
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
		const limitError = lowerPriceLimitError(bounds, lowerPrice);
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
	// hourly-to-daily range, then moves the step slider to that step.
	function commitGridCount(gridCount: string) {
		const nextInput = { ...committedInput, gridCount };
		const estimate = calculateSpotGridInput(
			nextInput,
			form.values.gridType,
		)?.estimate;
		if (!estimate || !hasHourlyVolatility) {
			setCommittedInput(nextInput);
			return;
		}

		const stepPercent = spotGridMinimumStepPercent(estimate);
		const maxStepPercent = hasDailyVolatility ? dailyRangeValue : null;
		if (
			stepPercent < hourlyRangeValue ||
			(maxStepPercent !== null && stepPercent > maxStepPercent)
		) {
			form.setFieldError(
				"gridCount",
				`Minimum grid step would be ${formatRangePercent(stepPercent)}; it must be ${
					maxStepPercent === null
						? `at least ${formatRangePercent(hourlyRangeValue)}`
						: `from ${formatRangePercent(hourlyRangeValue)} to ${formatRangePercent(maxStepPercent)}`
				}`,
			);
			return;
		}
		form.setFieldValue("rangePercent", stepPercent);
		setCommittedInput(nextInput);
	}

	function commitField(field: InputField) {
		const value = form.getValues()[field];
		if (value === committedInput[field]) return;

		if (field === "upperPrice") commitUpperPrice(value);
		else if (field === "lowerPrice") commitLowerPrice(value);
		else if (field === "gridCount") commitGridCount(value);
		else setCommittedInput({ ...committedInput, [field]: value });
	}

	function inputProps(field: InputField) {
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

	return (
		<Paper
			component="section"
			aria-busy={disabled || undefined}
			aria-labelledby="spot-grid-estimator-heading"
			p={paperPadding}
		>
			<Stack gap="md">
				<Center>
					<Title id="spot-grid-estimator-heading" order={2} size="h3">
						Spot Grid Calculator
					</Title>
				</Center>
				<SegmentedControl
					aria-label="Grid type"
					data={[
						{ label: "Arithmetic", value: "arithmetic" },
						{ label: "Geometric", value: "geometric" },
					]}
					disabled={disabled}
					fullWidth
					onChange={(value) => changeGridType(value as SpotGridType)}
					value={form.values.gridType}
				/>
				<Stack gap="sm">
					<SliderField
						disabled={disabled || !hasAnchor}
						formatValue={formatRangePercent}
						label="Upper price markup"
						max={bounds.upperMarkupMax}
						min={0}
						onChange={changeMarkup}
						precision={2}
						scaleLabels={[
							{ label: "0%", position: 0 },
							...(bounds.upperMarkupMax > DEFAULT_MARKUP_PERCENT
								? [
										{
											label: formatRangePercent(DEFAULT_MARKUP_PERCENT),
											position:
												(DEFAULT_MARKUP_PERCENT / bounds.upperMarkupMax) * 100,
										},
									]
								: []),
							{
								label: formatRangePercent(bounds.upperMarkupMax),
								position: 100,
							},
						]}
						step={0.01}
						value={form.values.markup}
					/>
					<SliderField
						disabled={disabled || !hasAnchor}
						formatValue={formatRangePercent}
						label="Lower price markup"
						max={bounds.lowerMarkupMax}
						min={0}
						onChange={changeLowerMarkup}
						precision={2}
						scaleLabels={[
							{ label: "0%", position: 0 },
							{
								label: formatRangePercent(bounds.lowerMarkupMax),
								position: 100,
							},
						]}
						step={0.01}
						value={form.values.lowerMarkup}
					/>
					<SliderField
						disabled={disabled || !canSelectRange}
						formatValue={formatRangePercent}
						label="Minimum grid step"
						max={canSelectRange ? dailyRangeValue : selectedRangePercent + 1}
						min={selectedRangePercent > 0 ? hourlyRangeValue : 0}
						onChange={changeRange}
						scaleLabels={[
							{ label: "Hourly range", position: 0 },
							{ label: "Daily range", position: 100 },
						]}
						step={
							canSelectRange ? (dailyRangeValue - hourlyRangeValue) / 100 : 1
						}
						value={form.values.rangePercent}
					/>
					{!hasAnchor ? (
						<Text c="dimmed" size="sm">
							Price markups need the current price.
						</Text>
					) : null}
					{hasAnchor && minPrice !== null && maxPrice !== null ? (
						<Text c="dimmed" size="sm">
							Binance grid bots accept prices from {minPrice} to {maxPrice} USDT
							(5-minute average price {formatNumber(anchor)} USDT).
						</Text>
					) : null}
					{gridLimits === null ? (
						<Text c="dimmed" size="sm">
							Binance grid limits are unavailable, so markups start from the
							latest hourly close and the prices are not checked.
						</Text>
					) : null}
					{hasAnchor && !hasHourlyVolatility ? (
						<Text c="dimmed" size="sm">
							Hourly range is unavailable, so the grid count is not derived from
							it.
						</Text>
					) : null}
					{hasHourlyVolatility && !hasDailyVolatility ? (
						<Text c="dimmed" size="sm">
							Daily range is unavailable, so the grid range stays hourly.
						</Text>
					) : null}
				</Stack>
				<SimpleGrid cols={2} spacing="md">
					<TextInput
						disabled={disabled}
						inputMode="decimal"
						label="Lower price (USDT)"
						required
						{...inputProps("lowerPrice")}
					/>
					<TextInput
						disabled={disabled}
						inputMode="decimal"
						label="Upper price (USDT)"
						required
						{...inputProps("upperPrice")}
					/>
					<TextInput
						disabled={disabled}
						inputMode="numeric"
						label="Grid count"
						required
						{...inputProps("gridCount")}
					/>
					<TextInput
						disabled={disabled}
						inputMode="decimal"
						label="USDT investment"
						required
						{...inputProps("investment")}
					/>
				</SimpleGrid>
				{calculation?.error ? (
					<Alert color="red" title="Check calculator inputs">
						{calculation.error}
					</Alert>
				) : null}
				<Stack aria-live="polite" gap="md">
					<ValueGroup
						title="Grid"
						items={[
							{ label: "Average price", value: values.averageEntryPrice },
							{ label: "Grid step", value: values.gridStepPercent },
						]}
					/>
					<SegmentedValueGroup
						title="Profit per trade"
						rows={values.profitSplits.map((split) => ({
							ariaLabel: `${split.label}: fees ${split.feeCost}, ${split.feeShareOfGross} profit; ${split.isLoss ? "net loss" : "clean profit"} ${split.cleanProfit}, ${split.cleanReturnPercent}`,
							items: [
								{
									color: "orange",
									label: "Fees",
									secondaryValue: split.feeShareOfGross,
									value: split.feeCost,
								},
								{
									color: split.isLoss ? "red" : "green",
									label: split.isLoss ? "Net loss" : "Profit",
									secondaryValue: split.cleanReturnPercent,
									value: split.cleanProfit,
								},
							],
							key: split.label,
							label: split.label === "Every trade" ? undefined : split.label,
							segments: [
								{
									color: "var(--mantine-color-orange-6)",
									key: "fees",
									percentage: split.feeSegmentPercent,
								},
								{
									color: "var(--mantine-color-green-6)",
									key: "clean-profit",
									percentage: split.cleanSegmentPercent,
								},
							],
						}))}
					/>
				</Stack>
			</Stack>
		</Paper>
	);
}
