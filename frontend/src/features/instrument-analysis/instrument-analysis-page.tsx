import {
	Box,
	Button,
	Center,
	Container,
	Group,
	Loader,
	LoadingOverlay,
	NumberInput,
	Paper,
	SimpleGrid,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { ApiError, fetchInstrumentAnalysis } from "../../api/client";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { getTelegramInitData, useTelegramBackButton } from "../../app/telegram";
import { AnalysisCriteriaFields } from "../analysis/analysis-criteria-fields";
import { defaultPeriodForUnit } from "../analysis/criteria";
import { useAnalysisErrorNotification } from "../analysis/use-analysis-error-notification";
import {
	type MarketScanCriteria,
	type MarketScanDraft,
	percentileCriterionSelection,
	percentileEvaluation,
	validateMarketScanCriteria,
} from "../market-scan/criteria";
import { formatRangePercent } from "../market-scan/results";
import { formatUtcCoverageDate } from "./presentation";

interface InstrumentAnalysisPageProps {
	committedCriteria: MarketScanCriteria;
	onBack(): void;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
	symbol: string;
}

export function InstrumentAnalysisPage({
	committedCriteria,
	onBack,
	onCommit,
	symbol,
}: InstrumentAnalysisPageProps) {
	const permission = useBusinessRequestPermission();
	const hasNativeBackButton = useTelegramBackButton(onBack);
	const form = useForm<MarketScanDraft>({
		initialValues: committedCriteria,
		mode: "controlled",
		validate: validateMarketScanCriteria,
		validateInputOnChange: true,
	});
	const setFormValues = form.setValues;
	const committedPeriod = committedCriteria.period;
	const committedUnit = committedCriteria.unit;
	const committedPercentile = committedCriteria.percentile;
	const committedMinimumRangePercent = committedCriteria.minimumRangePercent;
	const query = useQuery({
		enabled: permission.allowed,
		placeholderData: keepPreviousData,
		queryFn: async () => {
			const result = await fetchInstrumentAnalysis(
				symbol,
				[percentileCriterionSelection(committedCriteria)],
				{ initData: getTelegramInitData() },
			);
			if (!percentileEvaluation(result.evaluations)) {
				throw new ApiError("unexpected_error");
			}
			return result;
		},
		queryKey: instrumentAnalysisQueryKey(symbol, committedCriteria),
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
	});

	useEffect(() => {
		setFormValues({
			minimumRangePercent: committedMinimumRangePercent,
			percentile: committedPercentile,
			period: committedPeriod,
			unit: committedUnit,
		});
	}, [
		committedMinimumRangePercent,
		committedPeriod,
		committedPercentile,
		committedUnit,
		setFormValues,
	]);

	useAnalysisErrorNotification(query.error, "Instrument Analysis failed");

	const handleSubmit = form.onSubmit(async (values) => {
		if (
			typeof values.period !== "number" ||
			typeof values.percentile !== "number" ||
			typeof values.minimumRangePercent !== "number"
		) {
			return;
		}

		if (
			values.period === committedCriteria.period &&
			values.unit === committedCriteria.unit &&
			values.percentile === committedCriteria.percentile &&
			values.minimumRangePercent === committedCriteria.minimumRangePercent
		) {
			await query.refetch();
			return;
		}

		await onCommit({
			minimumRangePercent: values.minimumRangePercent,
			percentile: values.percentile,
			period: values.period,
			unit: values.unit,
		});
	});
	const submissionDisabled =
		!form.isValid() || !permission.allowed || query.isFetching;
	const result = query.data;
	const evaluation = result && percentileEvaluation(result.evaluations);

	return (
		<Container maw={720} px={0} size="sm">
			<Stack gap="md">
				{hasNativeBackButton ? null : (
					<Button onClick={onBack} variant="subtle">
						Back to Market Scan
					</Button>
				)}

				<Stack gap={2}>
					<Title order={1} size="h2">
						Instrument Analysis
					</Title>
					<Text c="dimmed" size="sm">
						{symbol}
					</Text>
				</Stack>

				<Paper component="form" onSubmit={handleSubmit} p="md" withBorder>
					<Stack gap="sm">
						<AnalysisCriteriaFields
							percentileInputProps={form.getInputProps("percentile")}
							percentileKey={form.key("percentile")}
							periodInputProps={form.getInputProps("period")}
							periodKey={form.key("period")}
							unit={form.values.unit}
							onUnitChange={(unit) => {
								form.setValues({ unit, period: defaultPeriodForUnit(unit) });
							}}
						/>
						<NumberInput
							decimalScale={10}
							key={form.key("minimumRangePercent")}
							label="Minimum Range"
							min={0}
							step={0.1}
							suffix="%"
							{...form.getInputProps("minimumRangePercent")}
						/>
						<Button
							disabled={submissionDisabled}
							loading={query.isFetching}
							type="submit"
						>
							Recalculate Instrument
						</Button>
					</Stack>
				</Paper>

				{query.isFetching && !query.data ? (
					<Center mih={180}>
						<Loader aria-label="Loading Instrument Analysis" />
					</Center>
				) : null}

				{result && evaluation ? (
					<Box pos="relative">
						<LoadingOverlay
							loaderProps={{ "aria-label": "Refreshing Instrument Analysis" }}
							overlayProps={{ blur: 1 }}
							visible={query.isFetching}
							zIndex={10}
						/>
						<Paper p="md" withBorder>
							<Stack gap="md">
								<Group justify="space-between">
									<Text c="dimmed" size="sm">
										Symbol
									</Text>
									<Text fw={700}>{result.symbol}</Text>
								</Group>
								<Group justify="space-between">
									<Text c="dimmed" size="sm">
										Matched
									</Text>
									<Text fw={700}>{result.matched ? "Yes" : "No"}</Text>
								</Group>
								<SimpleGrid cols={{ base: 1, xs: 2 }} spacing="sm">
									<ResultValue
										label="Range"
										value={formatRangePercent(evaluation.rangePercent)}
									/>
									<ResultValue
										label="Candle Count"
										value={String(evaluation.candleCount)}
									/>
									<ResultValue
										label="Coverage From"
										value={formatUtcCoverageDate(evaluation.from)}
									/>
									<ResultValue
										label="Coverage To"
										value={formatUtcCoverageDate(evaluation.to)}
									/>
								</SimpleGrid>
							</Stack>
						</Paper>
					</Box>
				) : null}
			</Stack>
		</Container>
	);
}

function instrumentAnalysisQueryKey(
	symbol: string,
	criteria: MarketScanCriteria,
) {
	return [
		"instrument-analysis",
		symbol,
		criteria.period,
		criteria.unit,
		criteria.percentile,
		criteria.minimumRangePercent,
	] as const;
}

function ResultValue({ label, value }: { label: string; value: string }) {
	return (
		<Stack gap={2}>
			<Text c="dimmed" size="xs">
				{label}
			</Text>
			<Text fw={600}>{value}</Text>
		</Stack>
	);
}
