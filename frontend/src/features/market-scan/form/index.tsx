import {
	Button,
	Fieldset,
	NumberInput,
	Paper,
	SimpleGrid,
	Stack,
	useMatches,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { AnalysisCriteriaFields } from "@/features/analysis/analysis-criteria-fields";
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
	const inputSize = useMatches({ base: "xs", sm: "sm" });
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
		<Paper component="form" onSubmit={handleSubmit} p={paperPadding} withBorder>
			<Stack gap={contentSpacing}>
				<Fieldset legend="Daily Volatility">
					<SimpleGrid cols={{ base: 1, xs: 3 }} spacing={contentSpacing}>
						<AnalysisCriteriaFields
							percentileInputProps={form.getInputProps("percentile")}
							percentileKey={form.key("percentile")}
							periodInputProps={form.getInputProps("period")}
							periodKey={form.key("period")}
							inputSize={inputSize}
							unit="days"
						/>
						<NumberInput
							decimalScale={10}
							key={form.key("minimumRangePercent")}
							label="Minimum Range"
							min={marketScanCriteriaConstraints.minimumRangePercent.minimum}
							size={inputSize}
							step={0.1}
							suffix="%"
							{...form.getInputProps("minimumRangePercent")}
						/>
					</SimpleGrid>
				</Fieldset>
				<Fieldset legend="Hourly Volatility">
					<SimpleGrid cols={{ base: 1, xs: 3 }} spacing={contentSpacing}>
						<AnalysisCriteriaFields
							percentileInputProps={form.getInputProps("hourlyPercentile")}
							percentileKey={form.key("hourlyPercentile")}
							periodInputProps={form.getInputProps("hourlyPeriod")}
							periodKey={form.key("hourlyPeriod")}
							inputSize={inputSize}
							unit="hours"
						/>
						<NumberInput
							decimalScale={10}
							key={form.key("hourlyMinimumRangePercent")}
							label="Minimum Range"
							min={marketScanCriteriaConstraints.minimumRangePercent.minimum}
							size={inputSize}
							step={0.1}
							suffix="%"
							{...form.getInputProps("hourlyMinimumRangePercent")}
						/>
					</SimpleGrid>
				</Fieldset>
				<Fieldset legend="Market Cap">
					<NumberInput
						decimalScale={2}
						key={form.key("minimumMarketCapMillions")}
						label="Minimum Market Cap (millions)"
						min={marketScanCriteriaConstraints.minimumMarketCapMillions.minimum}
						size={inputSize}
						prefix="$"
						step={1}
						suffix="M"
						{...form.getInputProps("minimumMarketCapMillions")}
					/>
				</Fieldset>
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
