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
import {
	criterionKeys,
	evaluationMetricKeys,
} from "@/api/analysis-identifiers";
import type { CriterionSelection } from "@/api/client";
import { useInstrumentAnalysisQuery } from "@/api/instrument-analysis";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useTelegramBackButton } from "@/app/telegram";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { formatMarketCapUsd, marketCapEvaluation } from "@/utils/market-cap";
import { formatRangePercent } from "@/utils/range-percent";

const rangeStatistics = [
	{
		key: criterionKeys.dailyVolatility,
		label: "Daily Range",
		coverageLabel: "Дней доступно",
	},
	{
		key: criterionKeys.hourlyVolatility,
		label: "Hourly Range",
		coverageLabel: "Часов доступно",
	},
];

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
	const textSize = useMatches({ base: "sm", sm: "md" });
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
								<Group justify="space-between" wrap="nowrap">
									<Text size={textSize}>Symbol</Text>
									<Text fw={700} size={textSize} ta="right">
										{result.symbol}
									</Text>
								</Group>
								{marketCap ? (
									<Group justify="space-between" wrap="nowrap">
										<Text size={textSize}>Market Cap</Text>
										<Text fw={700} size={textSize} ta="right">
											{formatMarketCapUsd(marketCap.marketCapUsd)}
										</Text>
									</Group>
								) : null}
								{rangeStatistics.map(({ key, label, coverageLabel }) => {
									const evaluation = result.evaluations.find(
										(item) => item.key === key,
									);
									const period = criterionSelections.find(
										(item) => item.key === key,
									)?.parameters.period;
									const range =
										evaluation?.metrics[evaluationMetricKeys.rangePercent];
									const count = evaluation?.candle_count;
									const coverage =
										typeof count === "number" &&
										Number.isInteger(count) &&
										count >= 0 &&
										typeof period === "number" &&
										Number.isInteger(period) &&
										period > 0
											? `${Math.min(count, period)} из ${period}`
											: "—";
									return (
										<Stack gap={contentSpacing} key={key}>
											<Group justify="space-between" wrap="nowrap">
												<Text size={textSize}>{label}</Text>
												<Text fw={700} size={textSize} ta="right">
													{typeof range === "number" && Number.isFinite(range)
														? formatRangePercent(range)
														: "—"}
												</Text>
											</Group>
											<Group justify="space-between" wrap="nowrap">
												<Text size={textSize}>{`${coverageLabel}: `}</Text>
												<Text fw={700} size={textSize} ta="right">
													{coverage}
												</Text>
											</Group>
										</Stack>
									);
								})}
							</Stack>
						</Paper>
					</RefreshingOverlay>
				) : null}
			</Stack>
		</Container>
	);
}
