import {
	Button,
	Paper,
	SimpleGrid,
	type SimpleGridProps,
	Stack,
	useMatches,
} from "@mantine/core";
import { useSelector } from "@tanstack/react-form";
import { type FormEvent, useId } from "react";

import { useAppForm } from "@/components/form/use-app-form";
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
	// Grid columns of the fieldsets; one per row by default.
	columns?: SimpleGridProps["cols"];
	initialCriteria: MarketScanCriteria;
	disabled: boolean;
	isSubmitting: boolean;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
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
	const volatilityPresets = applicationConfig.volatility;
	const id = useId();
	// The inputs may hold strings while being typed.
	const defaultValues: MarketScanDraft = initialCriteria;
	const form = useAppForm({ defaultValues });
	// The fields and the Run Market Scan button follow the values as they are
	// typed; the button stays disabled while they are invalid.
	const values = useSelector(form.store, (state) => state.values);
	const errors = validateMarketScanCriteria(values);
	const draftCriteria = criteriaFromValidDraft(values);
	const field = (name: keyof MarketScanDraft) => ({
		error: errors[name],
		id: `${id}-${name}`,
		onChange: (value: number | string) => form.setFieldValue(name, value),
		value: values[name],
	});

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!draftCriteria) return;
		await onCommit(draftCriteria);
	};

	return (
		<Paper component="form" onSubmit={handleSubmit} p={paperPadding}>
			<Stack gap={contentSpacing}>
				<SimpleGrid cols={columns} spacing={contentSpacing}>
					<VolatilityFieldset
						minimumRangePercent={field("minimumRangePercent")}
						minimumRangePresets={volatilityPresets.days.candleRangePresets}
						percentile={field("percentile")}
						percentilePresets={volatilityPresets.percentilePresets}
						period={field("period")}
						periodPresets={volatilityPresets.days.periodPresets}
						title="Daily Volatility"
						unit="days"
					/>
					<VolatilityFieldset
						minimumRangePercent={field("hourlyMinimumRangePercent")}
						minimumRangePresets={volatilityPresets.hours.candleRangePresets}
						percentile={field("hourlyPercentile")}
						percentilePresets={volatilityPresets.percentilePresets}
						period={field("hourlyPeriod")}
						periodPresets={volatilityPresets.hours.periodPresets}
						title="Hourly Volatility"
						unit="hours"
					/>
					<NumberInputFieldset
						inputs={[
							{
								decimalScale: 2,
								...field("minimumMarketCapMillions"),
								label: "Minimum Market Cap",
								min: marketScanCriteriaConstraints.minimumMarketCapMillions
									.minimum,
								presets: applicationConfig.marketCap.presets.map((value) => ({
									label: formatMarketCapPreset(value),
									value,
								})),
								step: 1,
							},
						]}
						title="Market Cap"
					/>
				</SimpleGrid>
				<Button
					disabled={disabled || !draftCriteria}
					loading={isSubmitting}
					type="submit"
				>
					Run Market Scan
				</Button>
			</Stack>
		</Paper>
	);
}
