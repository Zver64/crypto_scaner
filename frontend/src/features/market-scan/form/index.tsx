import { Button, Paper, Stack, useMatches } from "@mantine/core";
import { useForm } from "@mantine/form";
import { NumberInputFieldset } from "@/components/number-input-fieldset";
import {
	analysisCriteriaConstraints,
	maximumPeriodForUnit,
} from "@/features/analysis/criteria";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";
import {
	criteriaAreEqual,
	criteriaFromValidDraft,
} from "@/features/market-scan/form/utils";
import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	type MarketScanDraft,
	validateMarketScanCriteria,
} from "@/features/market-scan/pipeline";

const dailyPeriodPresets = [15, 30, 60] as const;
const dailyMinimumRangePresets = [5, 10] as const;
const hourlyMinimumRangePresets = [1, 2, 2.5, 3] as const;
const marketCapPresets = [100, 500, 1000] as const;
const percentilePresets = [75, 80, 90] as const;

function formatMarketCapPreset(value: (typeof marketCapPresets)[number]) {
	return value === 1000 ? "1B" : `${value}M`;
}

interface MarketScanFormProps {
	committedCriteria: MarketScanCriteria | undefined;
	disabled: boolean;
	isSubmitting: boolean;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
	onRefresh(): Promise<unknown>;
}

export function MarketScanForm({
	committedCriteria,
	disabled,
	isSubmitting,
	onCommit,
	onRefresh,
}: MarketScanFormProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const inputSize = "md";
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	const form = useForm<MarketScanDraft>({
		initialValues: committedCriteria ?? defaultMarketScanCriteria,
		mode: "controlled",
		validate: validateMarketScanCriteria,
		validateInputOnChange: true,
	});

	const handleSubmit = form.onSubmit(async (values) => {
		const criteria = criteriaFromValidDraft(values);
		if (!criteria) return;

		if (committedCriteria && criteriaAreEqual(criteria, committedCriteria)) {
			await onRefresh();
			return;
		}

		await onCommit(criteria);
	});

	return (
		<Paper component="form" onSubmit={handleSubmit} p={paperPadding}>
			<Stack gap={contentSpacing}>
				<NumberInputFieldset
					inputs={[
						{
							allowDecimal: false,
							error: form.errors.period,
							id: form.key("period"),
							label: "Analysis Period (days)",
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
							label: "Range Percentile",
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
							label: "Minimum Range (%)",
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
							label: "Analysis Period (hours)",
							max: maximumPeriodForUnit("hours"),
							min: analysisCriteriaConstraints.period.minimum,
							onChange: (value) => form.setFieldValue("hourlyPeriod", value),
							size: inputSize,
							value: form.values.hourlyPeriod,
						},
						{
							allowDecimal: false,
							error: form.errors.hourlyPercentile,
							id: form.key("hourlyPercentile"),
							label: "Range Percentile",
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
							label: "Minimum Range (%)",
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
							label: "Minimum Market Cap (USD millions)",
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
					title="Market Cap"
				/>
				<Button
					disabled={disabled || !form.isValid()}
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
