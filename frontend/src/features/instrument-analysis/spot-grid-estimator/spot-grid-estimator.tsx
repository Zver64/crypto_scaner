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
	gridCountForStep,
	LOWER_MARKUP_MAX_PERCENT,
	latestAvailableCandle,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	recommendedUpperPrice,
	type SpotGridType,
	spotGridEstimateValues,
	spotGridMinimumStepPercent,
	spotGridRecommendation,
	upperMarkupPercent,
} from "@/features/instrument-analysis/spot-grid-estimator/utils";
import type { SpotGridInput } from "@/utils/calculator/spot-grid";
import { formatRangePercent } from "@/utils/range-percent";

interface SpotGridEstimatorProps {
	candles?: readonly (PriceCandle | null)[];
	dailyVolatilityPercent?: number;
	disabled?: boolean;
	hourlyVolatilityPercent?: number;
	paperPadding: string;
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
	lowerMarkup: number | null,
	hourlyRangePercent: number | undefined,
): SpotGridFormValues {
	return {
		...input,
		gridType: "geometric",
		lowerMarkup: lowerMarkup ?? 0,
		markup: DEFAULT_MARKUP_PERCENT,
		rangePercent: hourlyRangePercent ?? 0,
	};
}

export function SpotGridEstimator({
	candles,
	dailyVolatilityPercent,
	disabled = false,
	hourlyVolatilityPercent,
	paperPadding,
}: SpotGridEstimatorProps) {
	const [recommendation] = useState(() =>
		spotGridRecommendation(candles, hourlyVolatilityPercent),
	);
	const form = useForm<SpotGridFormValues>({
		initialValues: formValues(
			recommendation.input,
			recommendation.lowerMarkup,
			hourlyVolatilityPercent,
		),
		mode: "controlled",
	});
	const [committedInput, setCommittedInput] = useState<SpotGridInput>(
		recommendation.input,
	);

	const latestHigh = latestAvailableCandle(candles)?.high;
	const hasLatestHigh =
		typeof latestHigh === "number" &&
		Number.isFinite(latestHigh) &&
		latestHigh > 0;
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
						extraValues.lowerMarkup ?? form.values.lowerMarkup,
					) ?? "")
				: committedInput.gridCount;
		form.setValues({ ...range, ...extraValues, gridCount });
		form.clearFieldError("gridCount");
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
		const upperPrice = recommendedUpperPrice(latestHigh, markup);
		if (!upperPrice) return;

		const lowerPrice =
			lowerPriceFromMarkup(upperPrice, form.values.lowerMarkup) ?? "";
		applyRange({ ...currentRange(), lowerPrice, upperPrice }, { markup });
	}

	function changeLowerMarkup(lowerMarkup: number) {
		const lowerPrice = lowerPriceFromMarkup(
			committedInput.upperPrice,
			lowerMarkup,
		);
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
		const lowerPrice = lowerPriceFromMarkup(
			upperPrice,
			form.values.lowerMarkup,
		);
		if (!lowerPrice) {
			setCommittedInput({ ...committedInput, upperPrice });
			return;
		}
		const markup = upperMarkupPercent(latestHigh, upperPrice);
		applyRange(
			{ ...currentRange(), lowerPrice, upperPrice },
			markup === null ? {} : { markup },
		);
	}

	function commitLowerPrice(lowerPrice: string) {
		const lowerMarkup = lowerMarkupPercent(
			committedInput.upperPrice,
			lowerPrice,
		);
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
						disabled={disabled || !hasLatestHigh}
						formatValue={formatRangePercent}
						label="Upper price markup"
						max={50}
						min={0}
						onChange={changeMarkup}
						scaleLabels={[
							{ label: "0%", position: 0 },
							{ label: "5%", position: 10 },
							{ label: "50%", position: 100 },
						]}
						step={1}
						value={form.values.markup}
					/>
					<SliderField
						disabled={disabled}
						formatValue={formatRangePercent}
						label="Lower price markup"
						max={LOWER_MARKUP_MAX_PERCENT}
						min={0}
						onChange={changeLowerMarkup}
						scaleLabels={[
							{ label: "0%", position: 0 },
							{ label: `${LOWER_MARKUP_MAX_PERCENT}%`, position: 100 },
						]}
						step={1}
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
					{!hasLatestHigh ? (
						<Text c="dimmed" size="sm">
							Upper price markup needs a valid hourly candle high.
						</Text>
					) : null}
					{hasLatestHigh && !hasHourlyVolatility ? (
						<Text c="dimmed" size="sm">
							Hourly range is unavailable, so no lower price recommendation can
							be made.
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
