import {
	Box,
	Button,
	Center,
	Container,
	Fieldset,
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
import { useAnalysisErrorNotification } from "../analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "../analysis/use-analysis-warning-notification";
import {
	marketCapEvaluation,
	volatilityEvaluation,
} from "../market-scan/criteria";
import {
	criterionSelections,
	dailyVolatilityKey,
	hourlyVolatilityKey,
	type MarketScanCriteria,
	type MarketScanDraft,
	validateMarketScanCriteria,
} from "../market-scan/pipeline";
import { formatMarketCapUsd, formatRangePercent } from "../market-scan/results";
import {
	formatUtcCoverageDate,
	hasRequiredInstrumentAnalysisEvaluations,
} from "./presentation";

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
	const committedPercentile = committedCriteria.percentile;
	const committedMinimumRangePercent = committedCriteria.minimumRangePercent;
	const committedHourlyPeriod = committedCriteria.hourlyPeriod;
	const committedHourlyPercentile = committedCriteria.hourlyPercentile;
	const committedHourlyMinimumRangePercent =
		committedCriteria.hourlyMinimumRangePercent;
	const committedMinimumMarketCapMillions =
		committedCriteria.minimumMarketCapMillions;
	const query = useQuery({
		enabled: permission.allowed,
		placeholderData: keepPreviousData,
		queryFn: async () => {
			const result = await fetchInstrumentAnalysis(
				symbol,
				criterionSelections(committedCriteria),
				{ initData: getTelegramInitData() },
			);
			if (
				!hasRequiredInstrumentAnalysisEvaluations(
					result.evaluations,
					committedCriteria.minimumMarketCapMillions > 0,
				)
			) {
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
			hourlyMinimumRangePercent: committedHourlyMinimumRangePercent,
			hourlyPercentile: committedHourlyPercentile,
			hourlyPeriod: committedHourlyPeriod,
			minimumMarketCapMillions: committedMinimumMarketCapMillions,
			minimumRangePercent: committedMinimumRangePercent,
			percentile: committedPercentile,
			period: committedPeriod,
		});
	}, [
		committedHourlyMinimumRangePercent,
		committedHourlyPercentile,
		committedHourlyPeriod,
		committedMinimumRangePercent,
		committedMinimumMarketCapMillions,
		committedPeriod,
		committedPercentile,
		setFormValues,
	]);

	useAnalysisErrorNotification(query.error, "Instrument Analysis failed");
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Instrument Analysis warning",
	);

	const handleSubmit = form.onSubmit(async (values) => {
		if (
			typeof values.period !== "number" ||
			typeof values.percentile !== "number" ||
			typeof values.minimumMarketCapMillions !== "number" ||
			typeof values.minimumRangePercent !== "number" ||
			typeof values.hourlyPeriod !== "number" ||
			typeof values.hourlyPercentile !== "number" ||
			typeof values.hourlyMinimumRangePercent !== "number"
		) {
			return;
		}

		if (
			values.period === committedCriteria.period &&
			values.percentile === committedCriteria.percentile &&
			values.minimumRangePercent === committedCriteria.minimumRangePercent &&
			values.hourlyPeriod === committedCriteria.hourlyPeriod &&
			values.hourlyPercentile === committedCriteria.hourlyPercentile &&
			values.hourlyMinimumRangePercent ===
				committedCriteria.hourlyMinimumRangePercent &&
			values.minimumMarketCapMillions ===
				committedCriteria.minimumMarketCapMillions
		) {
			await query.refetch();
			return;
		}

		await onCommit({
			hourlyMinimumRangePercent: values.hourlyMinimumRangePercent,
			hourlyPercentile: values.hourlyPercentile,
			hourlyPeriod: values.hourlyPeriod,
			minimumMarketCapMillions: values.minimumMarketCapMillions,
			minimumRangePercent: values.minimumRangePercent,
			percentile: values.percentile,
			period: values.period,
		});
	});
	const submissionDisabled =
		!form.isValid() || !permission.allowed || query.isFetching;
	const result = query.data;
	const daily =
		result && volatilityEvaluation(result.evaluations, dailyVolatilityKey);
	const hourly =
		result && volatilityEvaluation(result.evaluations, hourlyVolatilityKey);
	const marketCap = result && marketCapEvaluation(result.evaluations);
	const shortCircuitReason = hourly
		? "Hourly Volatility did not match."
		: "Daily Volatility did not match.";

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
									min={0}
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
									min={0}
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
								min={0}
								prefix="$"
								step={1}
								suffix="M"
								{...form.getInputProps("minimumMarketCapMillions")}
							/>
						</Fieldset>
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

				{result && daily ? (
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
								<VolatilityResult label="Daily Volatility" evaluation={daily} />
								{hourly ? (
									<VolatilityResult
										label="Hourly Volatility"
										evaluation={hourly}
									/>
								) : (
									<ShortCircuitedResult
										label="Hourly Volatility"
										reason="Daily Volatility did not match."
									/>
								)}
								{marketCap ? (
									<ResultValue
										label="Market Cap"
										value={formatMarketCapUsd(marketCap.marketCapUsd)}
									/>
								) : committedCriteria.minimumMarketCapMillions > 0 ? (
									<ShortCircuitedResult
										label="Market Cap"
										reason={shortCircuitReason}
									/>
								) : null}
							</Stack>
						</Paper>
					</Box>
				) : null}
			</Stack>
		</Container>
	);
}

export function instrumentAnalysisQueryKey(
	symbol: string,
	criteria: MarketScanCriteria,
) {
	return [
		"instrument-analysis",
		symbol,
		criteria.period,
		criteria.percentile,
		criteria.minimumRangePercent,
		criteria.hourlyPeriod,
		criteria.hourlyPercentile,
		criteria.hourlyMinimumRangePercent,
		criteria.minimumMarketCapMillions,
	] as const;
}

function VolatilityResult({
	evaluation,
	label,
}: {
	evaluation: NonNullable<ReturnType<typeof volatilityEvaluation>>;
	label: string;
}) {
	return (
		<Fieldset legend={label}>
			<SimpleGrid cols={{ base: 1, xs: 2 }} spacing="sm">
				<ResultValue
					label="Matched"
					value={evaluation.matched ? "Yes" : "No"}
				/>
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
		</Fieldset>
	);
}

function ShortCircuitedResult({
	label,
	reason,
}: {
	label: string;
	reason: string;
}) {
	return (
		<Fieldset legend={label}>
			<Text c="dimmed" size="sm">
				Not evaluated because {reason}
			</Text>
		</Fieldset>
	);
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
