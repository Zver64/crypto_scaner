import {
	Button,
	Center,
	Container,
	Group,
	Loader,
	Paper,
	Stack,
	Text,
} from "@mantine/core";
import { useInstrumentAnalysisQuery } from "@/api/instrument-analysis";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { useTelegramBackButton } from "../../app/telegram";
import { useAnalysisErrorNotification } from "../analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "../analysis/use-analysis-warning-notification";
import { marketCapEvaluation } from "../market-scan/criteria";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "../market-scan/pipeline";
import { formatMarketCapUsd } from "../market-scan/results";

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
	const query = useInstrumentAnalysisQuery(
		symbol,
		criterionSelections(committedCriteria),
		permission.allowed,
	);

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
					<RefreshingOverlay
						label="Refreshing Instrument Analysis"
						visible={query.isFetching}
					>
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
					</RefreshingOverlay>
				) : null}
			</Stack>
		</Container>
	);
}
