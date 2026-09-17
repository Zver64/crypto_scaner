import { Button, Paper, Stack, useMatches } from "@mantine/core";
import { useForm } from "@mantine/form";
import type { FormEvent } from "react";
import { NumberInputFieldset } from "@/components/number-input-fieldset";
import { applicationConfig } from "@/config";
import { VolatilityFieldset } from "@/features/analysis/volatility-form";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";
import { criteriaFromValidDraft } from "@/features/market-scan/form/utils";
import {
	type MarketScanCriteria,
	type MarketScanDraft,
	validateMarketScanCriteria,
} from "@/features/market-scan/pipeline";

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
	const volatilityPresets = applicationConfig.volatility;
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
				<VolatilityFieldset
					minimumRangePercent={{
						error: form.errors.minimumRangePercent,
						id: form.key("minimumRangePercent"),
						onChange: (value) =>
							form.setFieldValue("minimumRangePercent", value),
						value: form.values.minimumRangePercent,
					}}
					minimumRangePresets={volatilityPresets.days.candleRangePresets}
					percentile={{
						error: form.errors.percentile,
						id: form.key("percentile"),
						onChange: (value) => form.setFieldValue("percentile", value),
						value: form.values.percentile,
					}}
					percentilePresets={volatilityPresets.percentilePresets}
					period={{
						error: form.errors.period,
						id: form.key("period"),
						onChange: (value) => form.setFieldValue("period", value),
						value: form.values.period,
					}}
					periodPresets={volatilityPresets.days.periodPresets}
					size={inputSize}
					title="Daily Volatility"
					unit="days"
				/>
				<VolatilityFieldset
					minimumRangePercent={{
						error: form.errors.hourlyMinimumRangePercent,
						id: form.key("hourlyMinimumRangePercent"),
						onChange: (value) =>
							form.setFieldValue("hourlyMinimumRangePercent", value),
						value: form.values.hourlyMinimumRangePercent,
					}}
					minimumRangePresets={volatilityPresets.hours.candleRangePresets}
					percentile={{
						error: form.errors.hourlyPercentile,
						id: form.key("hourlyPercentile"),
						onChange: (value) => form.setFieldValue("hourlyPercentile", value),
						value: form.values.hourlyPercentile,
					}}
					percentilePresets={volatilityPresets.percentilePresets}
					period={{
						error: form.errors.hourlyPeriod,
						id: form.key("hourlyPeriod"),
						onChange: (value) => form.setFieldValue("hourlyPeriod", value),
						value: form.values.hourlyPeriod,
					}}
					periodPresets={volatilityPresets.hours.periodPresets}
					size={inputSize}
					title="Hourly Volatility"
					unit="hours"
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
							presets: applicationConfig.marketCap.presets.map((value) => ({
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
