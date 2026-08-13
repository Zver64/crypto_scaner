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
import { notifications } from "@mantine/notifications";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { ApiError, fetchInstrumentAnalysis } from "../../api/client";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { getTelegramInitData, useTelegramBackButton } from "../../app/telegram";
import { marketScanCriteriaConstraints } from "../market-scan/criteria";
import { formatRangePercent } from "../market-scan/results";
import {
	type InstrumentAnalysisCriteria,
	type InstrumentAnalysisDraft,
	validateInstrumentAnalysisCriteria,
} from "./criteria";
import { formatUtcCoverageDate } from "./presentation";

interface InstrumentAnalysisPageProps {
	committedCriteria: InstrumentAnalysisCriteria;
	onBack(): void;
	onCommit(criteria: InstrumentAnalysisCriteria): Promise<void>;
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
	const form = useForm<InstrumentAnalysisDraft>({
		initialValues: committedCriteria,
		mode: "controlled",
		validate: validateInstrumentAnalysisCriteria,
		validateInputOnChange: true,
	});
	const setFormValues = form.setValues;
	const committedPeriodDays = committedCriteria.periodDays;
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
			periodDays: committedPeriodDays,
		});
	}, [committedPeriodDays, committedPercentile, setFormValues]);

	useEffect(() => {
		if (!query.error) {
			return;
		}

		notifications.show({
			autoClose: 5000,
			color: "red",
			message:
				query.error instanceof ApiError
					? query.error.message
					: "An unexpected error occurred. Please try again.",
			title: "Instrument Analysis failed",
		});
	}, [query.error]);

	const handleSubmit = form.onSubmit(async (values) => {
		if (
			typeof values.periodDays !== "number" ||
			typeof values.percentile !== "number"
		) {
			return;
		}

		if (
			values.periodDays === committedCriteria.periodDays &&
			values.percentile === committedCriteria.percentile
		) {
			await query.refetch();
			return;
		}

		await onCommit({
			percentile: values.percentile,
			periodDays: values.periodDays,
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
						<NumberInput
							allowDecimal={false}
							key={form.key("periodDays")}
							label="Analysis Period"
							max={marketScanCriteriaConstraints.periodDays.maximum}
							min={marketScanCriteriaConstraints.periodDays.minimum}
							suffix=" days"
							{...form.getInputProps("periodDays")}
						/>
						<NumberInput
							allowDecimal={false}
							key={form.key("percentile")}
							label="Range Percentile"
							max={marketScanCriteriaConstraints.percentile.maximum}
							min={marketScanCriteriaConstraints.percentile.minimum}
							{...form.getInputProps("percentile")}
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
										label="Daily Range"
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
	criteria: InstrumentAnalysisCriteria,
) {
	return [
		"instrument-analysis",
		symbol,
		criteria.periodDays,
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
