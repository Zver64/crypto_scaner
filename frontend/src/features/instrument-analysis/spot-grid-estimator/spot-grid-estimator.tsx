import {
	Alert,
	Group,
	Paper,
	SegmentedControl,
	SimpleGrid,
	Slider,
	Stack,
	Text,
	TextInput,
	Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { type FocusEvent, type KeyboardEvent, useMemo, useState } from "react";
import type { PriceCandle } from "@/api/client";
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

interface SpotGridEstimatorProps {
	candles?: readonly (PriceCandle | null)[];
	disabled?: boolean;
	hourlyStepPercent?: number;
	paperPadding: string;
}

type SpotGridFormValues = SpotGridInput & {
	gridType: SpotGridType;
	markup: number;
};

type InputField = keyof SpotGridInput;

function formValues(input: SpotGridInput): SpotGridFormValues {
	return {
		...input,
		gridType: "geometric",
		markup: 5,
	};
}

function EstimateValue({ label, value }: { label: string; value: string }) {
	return (
		<Stack gap={2}>
			<Text fw={500} size="sm">
				{label}
			</Text>
			<Text fw={700}>{value}</Text>
		</Stack>
	);
}

export function SpotGridEstimator({
	candles,
	disabled = false,
	hourlyStepPercent,
	paperPadding,
}: SpotGridEstimatorProps) {
	const [recommendation] = useState(() =>
		spotGridRecommendation(candles, hourlyStepPercent),
	);
	const form = useForm<SpotGridFormValues>({
		initialValues: formValues(recommendation.input),
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
	const hasHourlyStep =
		typeof hourlyStepPercent === "number" &&
		Number.isFinite(hourlyStepPercent) &&
		hourlyStepPercent > 0;
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

	function commitMarkup(markup: number) {
		const upperPrice = recommendedUpperPrice(latestHigh, markup);
		if (!upperPrice) return;

		const lowerPrice =
			recommendedLowerPrice(
				upperPrice,
				hourlyStepPercent,
				committedInput.gridCount,
			) ?? "";
		form.setValues({ lowerPrice, markup, upperPrice });
		setCommittedInput({ ...committedInput, lowerPrice, upperPrice });
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
				<Title id="spot-grid-estimator-heading" order={2} size="h3">
					Spot Grid Calculator
				</Title>
				<SegmentedControl
					aria-label="Grid type"
					data={[
						{ label: "Arithmetic", value: "arithmetic" },
						{ label: "Geometric", value: "geometric" },
					]}
					disabled={disabled}
					fullWidth
					onChange={(value) =>
						form.setFieldValue("gridType", value as SpotGridType)
					}
					value={form.values.gridType}
				/>
				<Stack gap={4}>
					<Text fw={500} size="sm">
						Upper price markup: {form.values.markup}%
					</Text>
					<Stack gap={4}>
						<Slider
							disabled={disabled || !hasLatestHigh}
							label={(value) => `${value}%`}
							thumbLabel="Upper price markup"
							thumbValueText={(value) => `${value}%`}
							max={50}
							min={0}
							onChange={(value) => form.setFieldValue("markup", value)}
							onChangeEnd={commitMarkup}
							step={1}
							value={form.values.markup}
						/>
						<Group justify="space-between" wrap="nowrap">
							<Group justify="space-between" w="10%" wrap="nowrap">
								<Text c="dimmed" size="xs">
									0%
								</Text>
								<Text c="dimmed" size="xs">
									5%
								</Text>
							</Group>
							<Text c="dimmed" size="xs">
								50%
							</Text>
						</Group>
					</Stack>
					{!hasLatestHigh ? (
						<Text c="dimmed" size="sm">
							Upper price markup needs a valid hourly candle high.
						</Text>
					) : null}
					{hasLatestHigh && !hasHourlyStep ? (
						<Text c="dimmed" size="sm">
							Hourly Grid Step is unavailable, so no lower price recommendation
							can be made.
						</Text>
					) : null}
				</Stack>
				<SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
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
				<SimpleGrid cols={2} spacing="md" aria-live="polite">
					<EstimateValue
						label="Profit per step"
						value={`${values.profitPerStep}, ${values.profitPerStepPercent}`}
					/>
					<EstimateValue
						label="Average entry price"
						value={values.averageEntryPrice}
					/>
				</SimpleGrid>
			</Stack>
		</Paper>
	);
}
