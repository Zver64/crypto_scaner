import { Badge, Group, Stack, Text } from "@mantine/core";
import type {
	SecurityIssue,
	SecurityIssueChain,
	SecurityIssueSeverity,
} from "@/api/generated/models";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

const chainLabels: Record<SecurityIssueChain, string> = {
	bsc: "BSC",
	ethereum: "Ethereum",
	base: "Base",
	solana: "Solana",
};

const severityBadges: Record<
	SecurityIssueSeverity,
	{ color: string; label: string }
> = {
	risk: { color: "red", label: "Risk" },
	caution: { color: "yellow", label: "Caution" },
};

interface SecurityIssueListProps {
	issues: readonly SecurityIssue[];
}

// Failed Binance token audit checks grouped by chain, in audit order.
export function SecurityIssueList({ issues }: SecurityIssueListProps) {
	const { textSize } = useCoinPageLayout();
	const chains = [...new Set(issues.map((issue) => issue.chain))];
	return (
		<Stack gap="xs">
			<Text fw="bold" size={textSize}>
				Security Audit
			</Text>
			{chains.map((chain) => (
				<Stack gap="xs" key={chain}>
					<Badge color="gray" variant="light">
						{chainLabels[chain]}
					</Badge>
					{issues
						.filter((issue) => issue.chain === chain)
						.map((issue) => (
							<Stack gap={2} key={issue.title}>
								<Group gap="xs" wrap="nowrap">
									<Badge
										color={severityBadges[issue.severity].color}
										size="sm"
										variant="light"
									>
										{severityBadges[issue.severity].label}
									</Badge>
									<Text size={textSize}>{issue.title}</Text>
								</Group>
								<Text c="dimmed" size="xs">
									{issue.description}
								</Text>
							</Stack>
						))}
				</Stack>
			))}
		</Stack>
	);
}
