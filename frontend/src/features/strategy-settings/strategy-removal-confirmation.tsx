import { Button, Group, Modal, Text } from "@mantine/core";
import { useState } from "react";

interface StrategyRemovalConfirmationProps {
	isPending: boolean;
	// The strategy name; the dialog is open while it is set.
	name: string | undefined;
	onCancel(): void;
	onConfirm(): void;
}

export function StrategyRemovalConfirmation({
	isPending,
	name,
	onCancel,
	onConfirm,
}: StrategyRemovalConfirmationProps) {
	// The dialog fades out after name is cleared; keep showing the last one.
	const [shown, setShown] = useState(name);
	if (name !== undefined && name !== shown) setShown(name);
	return (
		<Modal
			centered
			onClose={onCancel}
			opened={name !== undefined}
			title={`Delete ${shown}?`}
		>
			<Text size="sm">No more alerts will be sent for this strategy.</Text>
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
