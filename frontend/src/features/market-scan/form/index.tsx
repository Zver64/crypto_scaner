import { Button, Paper, Stack, useMatches } from "@mantine/core";
import { useForm } from "@mantine/form";
import type { FormEvent } from "react";
import { NumberInputFieldset } from "@/components/number-input-fieldset";
import {
	analysisCriteriaConstraints,
	maximumPeriodForUnit,
} from "@/features/analysis/criteria";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";
import { criteriaFromValidDraft } from "@/features/market-scan/form/utils";
import {
	type MarketScanCriteria,
	type MarketScanDraft,
	validateMarketScanCriteria,
} from "@/features/market-scan/pipeline";

const dailyPeriodPresets = [15, 30, 60] as const;
const hourlyPeriodPresets = [24, 60, 100] as const;
const dailyMinimumRangePresets = [5, 10] as const;
const hourlyMinimumRangePresets = [1, 2, 2.5, 3] as const;
const marketCapPresets = [500, 1000, 5000, 10000] as const;
const percentilePresets = [75, 80, 90] as const;

function formatMarketCapPreset(value: number) {
	return value >= 1000 ? `${value / 1000}B` : `${value}M`;
}

interface MarketScanFormProps {
	initialCriteria: MarketScanCriteria;
	disabled: boolean;
	isSubmitting: boolean;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
}

export function MarketScanForm({
	initialCriteria,
	disabled,
	isSubmitting,
	onCommit,
}: MarketScanFormProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const inputSize = "md";
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	const form = useForm<MarketScanDraft>({
		initialValues: initialCriteria,
		mode: "controlled",
		validate: validateMarketScanCriteria,
		validateInputOnChange: true,
	});
	const draftCriteria = criteriaFromValidDraft(form.values);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!draftCriteria) return;
		await onCommit(draftCriteria);
	};

	return (
		<Paper component="form" onSubmit={handleSubmit} p={paperPadding}>
			<Stack gap={contentSpacing}>
				<NumberInputFieldset
					inputs={[
						{
							allowDecimal: false,
							error: form.errors.period,
							id: form.key("period"),
							label: "Period",
							max: maximumPeriodForUnit("days"),
							min: analysisCriteriaConstraints.period.minimum,
							onChange: (value) => form.setFieldValue("period", value),
							presets: dailyPeriodPresets.map((value) => ({
								label: String(value),
								value,
							})),
							size: inputSize,
							value: form.values.period,
						},
						{
							allowDecimal: false,
							error: form.errors.percentile,
							id: form.key("percentile"),
							label: "Percentile",
							max: analysisCriteriaConstraints.percentile.maximum,
							min: analysisCriteriaConstraints.percentile.minimum,
							onChange: (value) => form.setFieldValue("percentile", value),
							presets: percentilePresets.map((value) => ({
								label: String(value),
								value,
							})),
							size: inputSize,
							value: form.values.percentile,
						},
						{
							decimalScale: 10,
							error: form.errors.minimumRangePercent,
							id: form.key("minimumRangePercent"),
							label: "Candle Range (%)",
							min: marketScanCriteriaConstraints.minimumRangePercent.minimum,
							onChange: (value) =>
								form.setFieldValue("minimumRangePercent", value),
							presets: dailyMinimumRangePresets.map((value) => ({
								label: String(value),
								value,
							})),
							size: inputSize,
							step: 0.1,
							value: form.values.minimumRangePercent,
						},
					]}
					title="Daily Volatility"
				/>
				<NumberInputFieldset
					inputs={[
						{
							allowDecimal: false,
							error: form.errors.hourlyPeriod,
							id: form.key("hourlyPeriod"),
							label: "Period",
							max: maximumPeriodForUnit("hours"),
							min: analysisCriteriaConstraints.period.minimum,
							onChange: (value) => form.setFieldValue("hourlyPeriod", value),
							presets: hourlyPeriodPresets.map((value) => ({
								label: String(value),
								value,
							})),
							size: inputSize,
							value: form.values.hourlyPeriod,
						},
						{
							allowDecimal: false,
							error: form.errors.hourlyPercentile,
							id: form.key("hourlyPercentile"),
							label: "Percentile",
							max: analysisCriteriaConstraints.percentile.maximum,
							min: analysisCriteriaConstraints.percentile.minimum,
							onChange: (value) =>
								form.setFieldValue("hourlyPercentile", value),
							presets: percentilePresets.map((value) => ({
								label: String(value),
								value,
							})),
							size: inputSize,
							value: form.values.hourlyPercentile,
						},
						{
							decimalScale: 10,
							error: form.errors.hourlyMinimumRangePercent,
							id: form.key("hourlyMinimumRangePercent"),
							label: "Candle Range (%)",
							min: marketScanCriteriaConstraints.minimumRangePercent.minimum,
							onChange: (value) =>
								form.setFieldValue("hourlyMinimumRangePercent", value),
							presets: hourlyMinimumRangePresets.map((value) => ({
								label: String(value),
								value,
							})),
							size: inputSize,
							step: 0.1,
							value: form.values.hourlyMinimumRangePercent,
						},
					]}
					title="Hourly Volatility"
				/>
				<NumberInputFieldset
					inputs={[
						{
							decimalScale: 2,
							error: form.errors.minimumMarketCapMillions,
							id: form.key("minimumMarketCapMillions"),
							label: "Minimum Market Cap",
							min: marketScanCriteriaConstraints.minimumMarketCapMillions
								.minimum,
							onChange: (value) =>
								form.setFieldValue("minimumMarketCapMillions", value),
							presets: marketCapPresets.map((value) => ({
								label: formatMarketCapPreset(value),
								value,
							})),
							size: inputSize,
							step: 1,
							value: form.values.minimumMarketCapMillions,
						},
					]}
					presetsPosition="below"
					title="Market Cap"
				/>
				<Button
					disabled={disabled || !draftCriteria}
					loading={isSubmitting}
					size={inputSize}
					type="submit"
				>
					Run Market Scan
				</Button>
			</Stack>
		</Paper>
	);
}
