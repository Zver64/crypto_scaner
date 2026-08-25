import {
	Box,
	Button,
	Center,
	Container,
	Group,
	Loader,
	LoadingOverlay,
	Paper,
	SimpleGrid,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { fetchInstrumentAnalysis } from "../../api/client";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { getTelegramInitData, useTelegramBackButton } from "../../app/telegram";
import { AnalysisCriteriaFields } from "../analysis/analysis-criteria-fields";
import {
	type AnalysisCriteria,
	type AnalysisDraft,
	defaultPeriodForUnit,
	validateAnalysisCriteria,
} from "../analysis/criteria";
import { useAnalysisErrorNotification } from "../analysis/use-analysis-error-notification";
import { formatRangePercent } from "../market-scan/results";
import { formatUtcCoverageDate } from "./presentation";

interface InstrumentAnalysisPageProps {
	committedCriteria: AnalysisCriteria;
	onBack(): void;
	onCommit(criteria: AnalysisCriteria): Promise<void>;
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
	const form = useForm<AnalysisDraft>({
		initialValues: committedCriteria,
		mode: "controlled",
		validate: validateAnalysisCriteria,
		validateInputOnChange: true,
	});
	const setFormValues = form.setValues;
	const committedPeriod = committedCriteria.period;
	const committedUnit = committedCriteria.unit;
	const committedPercentile = committedCriteria.percentile;
	const query = useQuery({
		enabled: permission.allowed,
		placeholderData: keepPreviousData,
		queryFn: () =>
			fetchInstrumentAnalysis(symbol, committedCriteria, {
				initData: getTelegramInitData(),
			}),
		queryKey: instrumentAnalysisQueryKey(symbol, committedCriteria),
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
	});

	useEffect(() => {
		setFormValues({
			percentile: committedPercentile,
			period: committedPeriod,
			unit: committedUnit,
		});
	}, [committedPeriod, committedPercentile, committedUnit, setFormValues]);

	useAnalysisErrorNotification(query.error, "Instrument Analysis failed");

	const handleSubmit = form.onSubmit(async (values) => {
		if (
			typeof values.period !== "number" ||
			typeof values.percentile !== "number"
		) {
			return;
		}

		if (
			values.period === committedCriteria.period &&
			values.unit === committedCriteria.unit &&
			values.percentile === committedCriteria.percentile
		) {
			await query.refetch();
			return;
		}

		await onCommit({
			percentile: values.percentile,
			period: values.period,
			unit: values.unit,
		});
	});
	const submissionDisabled =
		!form.isValid() || !permission.allowed || query.isFetching;

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

				{query.data ? (
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
									<Text fw={700}>{query.data.symbol}</Text>
								</Group>
								<SimpleGrid cols={{ base: 1, xs: 2 }} spacing="sm">
									<ResultValue
										label={`${query.data.unit === "days" ? "Daily" : "Hourly"} Range`}
										value={formatRangePercent(query.data.range_percent)}
									/>
									<ResultValue
										label="Candle Count"
										value={String(query.data.candle_count)}
									/>
									<ResultValue
										label="Coverage From"
										value={formatUtcCoverageDate(query.data.from)}
									/>
									<ResultValue
										label="Coverage To"
										value={formatUtcCoverageDate(query.data.to)}
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
	criteria: AnalysisCriteria,
) {
	return [
		"instrument-analysis",
		symbol,
		criteria.period,
		criteria.unit,
		criteria.percentile,
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
