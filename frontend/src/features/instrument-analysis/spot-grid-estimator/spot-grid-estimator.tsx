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
import type { PriceCandle } from "@/api/client";
import { SliderField } from "@/components/slider-field";
import { ValueGroup } from "@/components/value-group";
import {
	calculateSpotGridInput,
	latestAvailableCandle,
	recommendedLowerPrice,
	recommendedUpperPrice,
	type SpotGridType,
	spotGridEstimateValues,
	spotGridRecommendation,
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
	markup: number;
	rangePercent: number;
};

type InputField = keyof SpotGridInput;

function formValues(
	input: SpotGridInput,
	hourlyRangePercent: number | undefined,
): SpotGridFormValues {
	return {
		...input,
		gridType: "geometric",
		markup: 5,
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
		initialValues: formValues(recommendation.input, hourlyVolatilityPercent),
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

	function commitField(field: InputField) {
		const nextInput = {
			...committedInput,
			[field]: form.getValues()[field],
		};
		setCommittedInput(nextInput);
	}

	function changeMarkup(markup: number) {
		const upperPrice = recommendedUpperPrice(latestHigh, markup);
		if (!upperPrice) return;

		const lowerPrice =
			recommendedLowerPrice(
				upperPrice,
				selectedRangePercent,
				committedInput.gridCount,
				form.values.gridType,
			) ?? "";
		form.setValues({ lowerPrice, markup, upperPrice });
		setCommittedInput({ ...committedInput, lowerPrice, upperPrice });
	}

	function changeRange(rangePercent: number) {
		const lowerPrice =
			recommendedLowerPrice(
				committedInput.upperPrice,
				rangePercent,
				committedInput.gridCount,
				form.values.gridType,
			) ?? "";
		form.setValues({ lowerPrice, rangePercent });
		setCommittedInput({ ...committedInput, lowerPrice });
	}

	function changeGridType(gridType: SpotGridType) {
		const lowerPrice =
			recommendedLowerPrice(
				committedInput.upperPrice,
				selectedRangePercent,
				committedInput.gridCount,
				gridType,
			) ?? "";
		form.setValues({ gridType, lowerPrice });
		setCommittedInput({ ...committedInput, lowerPrice });
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
						formatValue={(value) => `${value}%`}
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
				<SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md" aria-live="polite">
					<ValueGroup
						title="Profit"
						items={[
							{ label: "USDT (nominal)", value: values.profitPerStep },
							{ label: "Percent", value: values.profitPerStepPercent },
						]}
					/>
					<ValueGroup
						title="Grid info"
						items={[
							{ label: "Average price", value: values.averageEntryPrice },
							{ label: "Grid step", value: values.gridStepPercent },
						]}
					/>
				</SimpleGrid>
			</Stack>
		</Paper>
	);
}
