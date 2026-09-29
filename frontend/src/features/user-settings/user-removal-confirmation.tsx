import { Button, Group, Modal, Text } from "@mantine/core";
import { useState } from "react";

interface UserRemovalConfirmationProps {
	isPending: boolean;
	// The user being deleted; the dialog is open while it is set.
	name: string | undefined;
	onCancel(): void;
	onConfirm(): void;
}

export function UserRemovalConfirmation({
	isPending,
	name,
	onCancel,
	onConfirm,
}: UserRemovalConfirmationProps) {
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
			<Text size="sm">
				{shown} loses access to the scanner, and their favorites and alerts are
				deleted. The bot can add them again later.
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
