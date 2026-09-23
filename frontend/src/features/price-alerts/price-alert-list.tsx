import { ActionIcon, Group, Text, Tooltip } from "@mantine/core";
import { IconPencil, IconTrash } from "@tabler/icons-react";
import type { PriceAlert } from "@/api/generated/models";

interface PriceAlertListProps {
	alerts: readonly PriceAlert[];
	deletingID?: number;
	onDelete(alertID: number): void;
	onEdit(alert: PriceAlert): void;
}

export function PriceAlertList({
	alerts,
	deletingID,
	onDelete,
	onEdit,
}: PriceAlertListProps) {
	return alerts.map((alert) => (
		<Group justify="space-between" key={alert.id} wrap="nowrap">
			<Text ff="monospace" fw={600}>
				{alert.target} USDT
			</Text>
			<Group gap="xs" wrap="nowrap">
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
