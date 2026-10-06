import { ActionIcon, Group, Text, Tooltip } from "@mantine/core";
import { IconPencil, IconTrash } from "@tabler/icons-react";
import type { PriceAlert } from "@/api/generated/models";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { TargetChange } from "@/features/price-alerts/target-change";
import { sortAlertsDescending } from "@/features/price-alerts/utils";
import { formatNumber } from "@/utils/number-format";

interface PriceAlertListProps {
	alerts: readonly PriceAlert[];
	deletingID?: number;
	onDelete(alertID: number): void;
	onEdit(alert: PriceAlert): void;
	priceSource?: CoinChartData;
}

export function PriceAlertList({
	alerts,
	deletingID,
	onDelete,
	onEdit,
	priceSource,
}: PriceAlertListProps) {
	return sortAlertsDescending(alerts).map((alert) => (
		<Group justify="space-between" key={alert.id} wrap="nowrap">
			<Text ff="monospace" fw={600}>
				{formatNumber(alert.target)} USDT
			</Text>
			<Group gap="xs" wrap="nowrap">
				<TargetChange source={priceSource} target={alert.target} />
				<Tooltip label="Edit alert">
					<ActionIcon
						aria-label={`Edit ${alert.target} USDT alert`}
						onClick={() => onEdit(alert)}
						variant="subtle"
					>
						<IconPencil size={18} />
					</ActionIcon>
				</Tooltip>
				<Tooltip label="Delete alert">
					<ActionIcon
						aria-label={`Delete ${alert.target} USDT alert`}
						color="red"
						loading={deletingID === alert.id}
						onClick={() => onDelete(alert.id)}
						variant="subtle"
					>
						<IconTrash size={18} />
					</ActionIcon>
				</Tooltip>
			</Group>
		</Group>
	));
}
