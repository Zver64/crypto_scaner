import { Button, Group, Modal, Text } from "@mantine/core";

interface ScannerIndicatorRemovalConfirmationProps {
	isPending: boolean;
	onCancel(): void;
	onConfirm(): void;
	title: string | undefined;
}

export function ScannerIndicatorRemovalConfirmation({
	isPending,
	onCancel,
	onConfirm,
	title,
}: ScannerIndicatorRemovalConfirmationProps) {
	return (
		<Modal
			centered
			onClose={onCancel}
			opened={title !== undefined}
			title="Delete indicator?"
		>
			<Text size="sm">
				{title} will no longer be calculated, drawn on charts, or shown in
				tables.
			</Text>
			<Group justify="flex-end" mt="md">
				<Button disabled={isPending} onClick={onCancel} variant="default">
					Cancel
				</Button>
				<Button color="red" loading={isPending} onClick={onConfirm}>
					Delete
				</Button>
			</Group>
		</Modal>
	);
}
