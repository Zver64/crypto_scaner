import {
	Button,
	Center,
	Container,
	Group,
	Loader,
	Paper,
	Stack,
	Text,
	useMatches,
} from "@mantine/core";
import type { CriterionSelection } from "@/api/client";
import { useInstrumentAnalysisQuery } from "@/api/instrument-analysis";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useTelegramBackButton } from "@/app/telegram";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { formatMarketCapUsd, marketCapEvaluation } from "@/utils/market-cap";

interface InstrumentAnalysisScreenProps {
	criterionSelections: readonly CriterionSelection[];
	onBack(): void;
	symbol: string;
}

export function InstrumentAnalysisScreen({
	criterionSelections,
	onBack,
	symbol,
}: InstrumentAnalysisScreenProps) {
	const contentSpacing = useMatches({ base: "sm", sm: "md" });
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	const textSize = useMatches({ base: "xs", sm: "sm" });
	const permission = useBusinessRequestPermission();
	const hasNativeBackButton = useTelegramBackButton(onBack);
	const query = useInstrumentAnalysisQuery(
		symbol,
		criterionSelections,
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
			<Stack gap={contentSpacing}>
				{hasNativeBackButton ? null : (
					<Button onClick={onBack} size={textSize} variant="subtle">
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
						<Paper p={paperPadding} withBorder>
							<Stack gap={contentSpacing}>
								<Group justify="space-between">
									<Text c="dimmed" size={textSize}>
										Symbol
									</Text>
									<Text fw={700} size={textSize}>
										{result.symbol}
									</Text>
								</Group>
								{marketCap ? (
									<Group justify="space-between">
										<Text c="dimmed" size={textSize}>
											Market Cap
										</Text>
										<Text fw={700} size={textSize}>
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
