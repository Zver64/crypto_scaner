import {
	Box,
	Button,
	Center,
	Container,
	Group,
	Loader,
	LoadingOverlay,
	Paper,
	Stack,
	Text,
} from "@mantine/core";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ApiError, fetchInstrumentAnalysis } from "../../api/client";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { getTelegramInitData, useTelegramBackButton } from "../../app/telegram";
import { useAnalysisErrorNotification } from "../analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "../analysis/use-analysis-warning-notification";
import { marketCapEvaluation } from "../market-scan/criteria";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "../market-scan/pipeline";
import { formatMarketCapUsd } from "../market-scan/results";
import { hasRequiredInstrumentAnalysisEvaluations } from "./presentation";

interface InstrumentAnalysisPageProps {
	committedCriteria: MarketScanCriteria;
	onBack(): void;
	symbol: string;
}

export function InstrumentAnalysisPage({
	committedCriteria,
	onBack,
	symbol,
}: InstrumentAnalysisPageProps) {
	const permission = useBusinessRequestPermission();
	const hasNativeBackButton = useTelegramBackButton(onBack);
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

	useAnalysisErrorNotification(query.error, "Instrument Analysis failed");
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Instrument Analysis warning",
	);

	const result = query.data;
	const marketCap = result && marketCapEvaluation(result.evaluations);

	return (
		<Container maw={720} px={0} size="sm">
			<Stack gap="md">
				{hasNativeBackButton ? null : (
					<Button onClick={onBack} variant="subtle">
						Back to Market Scan
					</Button>
				)}

				{query.isFetching && !query.data ? (
					<Center mih={180}>
						<Loader aria-label="Loading Instrument Analysis" />
					</Center>
				) : null}

				{result ? (
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
								{marketCap ? (
									<Group justify="space-between">
										<Text c="dimmed" size="sm">
											Market Cap
										</Text>
										<Text fw={700}>
											{formatMarketCapUsd(marketCap.marketCapUsd)}
										</Text>
									</Group>
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
