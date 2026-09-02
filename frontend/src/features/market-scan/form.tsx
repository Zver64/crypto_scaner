import { Button, Fieldset, NumberInput, Paper, Stack } from "@mantine/core";
import { useForm } from "@mantine/form";
import { AnalysisCriteriaFields } from "@/features/analysis/analysis-criteria-fields";
import { marketScanCriteriaConstraints } from "./criteria";
import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	type MarketScanDraft,
	marketScanCriteriaIdentity,
	validateMarketScanCriteria,
} from "./pipeline";

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
		<Paper component="form" onSubmit={handleSubmit} p="md" withBorder>
			<Stack gap="sm">
				<Fieldset legend="Daily Volatility">
					<Stack gap="sm">
						<AnalysisCriteriaFields
							percentileInputProps={form.getInputProps("percentile")}
							percentileKey={form.key("percentile")}
							periodInputProps={form.getInputProps("period")}
							periodKey={form.key("period")}
							unit="days"
						/>
						<NumberInput
							decimalScale={10}
							key={form.key("minimumRangePercent")}
							label="Minimum Range"
							min={marketScanCriteriaConstraints.minimumRangePercent.minimum}
							step={0.1}
							suffix="%"
							{...form.getInputProps("minimumRangePercent")}
						/>
					</Stack>
				</Fieldset>
				<Fieldset legend="Hourly Volatility">
					<Stack gap="sm">
						<AnalysisCriteriaFields
							percentileInputProps={form.getInputProps("hourlyPercentile")}
							percentileKey={form.key("hourlyPercentile")}
							periodInputProps={form.getInputProps("hourlyPeriod")}
							periodKey={form.key("hourlyPeriod")}
							unit="hours"
						/>
						<NumberInput
							decimalScale={10}
							key={form.key("hourlyMinimumRangePercent")}
							label="Minimum Range"
							min={marketScanCriteriaConstraints.minimumRangePercent.minimum}
							step={0.1}
							suffix="%"
							{...form.getInputProps("hourlyMinimumRangePercent")}
						/>
					</Stack>
				</Fieldset>
				<Fieldset legend="Market Cap">
					<NumberInput
						decimalScale={2}
						key={form.key("minimumMarketCapMillions")}
						label="Minimum Market Cap (millions)"
						min={marketScanCriteriaConstraints.minimumMarketCapMillions.minimum}
						prefix="$"
						step={1}
						suffix="M"
						{...form.getInputProps("minimumMarketCapMillions")}
					/>
				</Fieldset>
				<Button
					disabled={disabled || !form.isValid()}
					loading={isSubmitting}
					type="submit"
				>
					Run Market Scan
				</Button>
			</Stack>
		</Paper>
	);
}

function criteriaAreEqual(
	left: MarketScanCriteria,
	right: MarketScanCriteria,
): boolean {
	const leftIdentity = marketScanCriteriaIdentity(left);
	const rightIdentity = marketScanCriteriaIdentity(right);
	return leftIdentity.every((value, index) => value === rightIdentity[index]);
}

function criteriaFromValidDraft(
	values: MarketScanDraft,
): MarketScanCriteria | undefined {
	if (
		typeof values.period !== "number" ||
		typeof values.percentile !== "number" ||
		typeof values.minimumMarketCapMillions !== "number" ||
		typeof values.minimumRangePercent !== "number" ||
		typeof values.hourlyPeriod !== "number" ||
		typeof values.hourlyPercentile !== "number" ||
		typeof values.hourlyMinimumRangePercent !== "number"
	) {
		return undefined;
	}

	return {
		hourlyMinimumRangePercent: values.hourlyMinimumRangePercent,
		hourlyPercentile: values.hourlyPercentile,
		hourlyPeriod: values.hourlyPeriod,
		minimumMarketCapMillions: values.minimumMarketCapMillions,
		minimumRangePercent: values.minimumRangePercent,
		percentile: values.percentile,
		period: values.period,
	};
}
