import { Button, Group, Modal, Text } from "@mantine/core";

interface FavoriteRemovalConfirmationProps {
	alertCount: number;
	isPending: boolean;
	onCancel(): void;
	onConfirm(): void;
	opened: boolean;
	symbol: string;
}

export function FavoriteRemovalConfirmation({
	alertCount,
	isPending,
	onCancel,
	onConfirm,
	opened,
	symbol,
}: FavoriteRemovalConfirmationProps) {
	return (
		<Modal
			centered
			onClose={onCancel}
			opened={opened}
			title="Remove favorite and alerts?"
		>
			<Text size="sm">
				Removing {symbol} will also delete {alertCount} active price{" "}
				{alertCount === 1 ? "alert" : "alerts"}.
			</Text>
			<Group justify="flex-end" mt="md">
				<Button disabled={isPending} onClick={onCancel} variant="default">
					Cancel
				</Button>
				<Button color="red" loading={isPending} onClick={onConfirm}>
					Remove
				</Button>
			</Group>
		</Modal>
	);
}
