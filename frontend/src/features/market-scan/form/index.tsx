import {
	Fieldset,
	Paper,
	SimpleGrid,
	type SimpleGridProps,
	Stack,
	useMatches,
} from "@mantine/core";

import { useAppForm } from "@/components/form";
import { applicationConfig } from "@/config";
import { VolatilityFields } from "@/features/analysis/volatility-form";
import { presetOptions } from "@/features/analysis/volatility-form/utils";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";
import type {
	MarketScanCriteria,
	MarketScanDraft,
} from "@/features/market-scan/pipeline";
import { marketScanCriteriaSchema } from "@/features/market-scan/schemas";

function formatMarketCapPreset(value: number) {
	return value >= 1000 ? `${value / 1000}B` : `${value}M`;
}

interface MarketScanFormProps {
	// Grid columns of the fieldsets; one per row by default.
	columns?: SimpleGridProps["cols"];
	initialCriteria: MarketScanCriteria;
	disabled: boolean;
	isSubmitting: boolean;
	onCommit(criteria: MarketScanCriteria): void;
}

export function MarketScanForm({
	columns = 1,
	initialCriteria,
	disabled,
	isSubmitting,
	onCommit,
}: MarketScanFormProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	// The daily and the hourly criterion, each with its own candle range.
	const volatilityCriteria = [
		{
			fields: { percentile: "percentile", period: "period" },
			legend: "Daily Volatility",
			rangeField: "minimumRangePercent",
			unit: "days",
		},
		{
			fields: { percentile: "hourlyPercentile", period: "hourlyPeriod" },
			legend: "Hourly Volatility",
			rangeField: "hourlyMinimumRangePercent",
			unit: "hours",
		},
	] as const;
	// The inputs may hold strings while being typed.
	const defaultValues: MarketScanDraft = initialCriteria;
	const form = useAppForm({
		defaultValues,
		onSubmit: ({ value }) => onCommit(marketScanCriteriaSchema.parse(value)),
		validators: { onChange: marketScanCriteriaSchema },
	});

	return (
		<Paper
			component="form"
			onSubmit={(event) => {
				event.preventDefault();
				void form.handleSubmit();
			}}
			p={paperPadding}
		>
			<Stack gap={contentSpacing}>
				<SimpleGrid cols={columns} spacing={contentSpacing}>
					{volatilityCriteria.map((criterion) => (
						<VolatilityFields
							fields={criterion.fields}
							form={form}
							key={criterion.legend}
							legend={criterion.legend}
							unit={criterion.unit}
						>
							<form.AppField name={criterion.rangeField}>
								{(field) => {
									const { candleRangeMaximum, candleRangePresets } =
										applicationConfig.volatility[criterion.unit];
									return (
										<field.SliderField
											label="Candle Range (%)"
											// The minimum range lets every candle pass.
											marks={[
												{
													label: "Off",
													value:
														marketScanCriteriaConstraints.minimumRangePercent
															.minimum,
												},
												...presetOptions(candleRangePresets),
											]}
											max={candleRangeMaximum}
											min={
												marketScanCriteriaConstraints.minimumRangePercent
													.minimum
											}
											precision={1}
											step={0.1}
										/>
									);
								}}
							</form.AppField>
						</VolatilityFields>
					))}
					<Fieldset legend="Market Cap">
						<form.AppField name="minimumMarketCapMillions">
							{(field) => (
								<field.NumberField
									decimalScale={2}
									label="Minimum Market Cap"
									min={
										marketScanCriteriaConstraints.minimumMarketCapMillions
											.minimum
									}
									presets={applicationConfig.marketCap.presets.map((value) => ({
										label: formatMarketCapPreset(value),
										value,
									}))}
									step={1}
								/>
							)}
						</form.AppField>
					</Fieldset>
				</SimpleGrid>
				<form.AppForm>
					<form.SubmitButton disabled={disabled} loading={isSubmitting}>
						Run Market Scan
					</form.SubmitButton>
				</form.AppForm>
			</Stack>
		</Paper>
	);
}
