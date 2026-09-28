import { Divider, Paper, Stack } from "@mantine/core";
import type { InstrumentAnalysisResponse } from "@/api/generated/models";
import { PercentChange } from "@/components/percent-change";
import { criterionKeys } from "@/features/analysis/identifiers";
import { OverviewRow } from "@/features/instrument-analysis/overview-row";
import { SecurityIssueList } from "@/features/instrument-analysis/security-issue-list";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { volatilityEvaluation } from "@/features/market-scan/criteria";
import { formatMarketCapUsd, marketCapEvaluation } from "@/utils/market-cap";
import { formatRangePercent } from "@/utils/range-percent";

const ranges = [
	{ key: criterionKeys.dailyVolatility, label: "Daily Range" },
	{ key: criterionKeys.hourlyVolatility, label: "Hourly Range" },
];

interface CoinOverviewProps {
	result: InstrumentAnalysisResponse | undefined;
	sevenDayChange: number | null;
}

export function CoinOverview({ result, sevenDayChange }: CoinOverviewProps) {
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	const evaluations = result?.evaluations ?? [];
	const marketCap = marketCapEvaluation(evaluations);
	return (
		<Paper p={paperPadding}>
			<Stack gap={contentSpacing}>
				{marketCap ? (
					<OverviewRow
						label="Market Cap"
						value={formatMarketCapUsd(marketCap.marketCapUsd)}
					/>
				) : null}
				{result ? (
					<OverviewRow
						label="7d change percent"
						value={<PercentChange value={sevenDayChange} />}
					/>
				) : null}
				{ranges.map(({ key, label }) => {
					const range = volatilityEvaluation(evaluations, key)?.rangePercent;
					return (
						<OverviewRow
							highlighted
							key={key}
							label={label}
							value={
								range !== undefined && Number.isFinite(range)
									? formatRangePercent(range)
									: "—"
							}
						/>
					);
				})}
				{result?.security_issues.length ? (
					<>
						<Divider />
						<SecurityIssueList issues={result.security_issues} />
					</>
				) : null}
			</Stack>
		</Paper>
	);
}
