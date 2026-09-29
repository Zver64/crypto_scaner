import { Button, Group, Modal, Text } from "@mantine/core";
import { useState } from "react";

interface ScannerIndicatorRemovalConfirmationProps {
	isPending: boolean;
	onCancel(): void;
	onConfirm(): void;
	// What is removed, such as "d-rsi" or "all indicators"; the dialog is open
	// while it is set.
	subject: string | undefined;
}

export function ScannerIndicatorRemovalConfirmation({
	isPending,
	onCancel,
	onConfirm,
	subject,
}: ScannerIndicatorRemovalConfirmationProps) {
	// The dialog fades out after subject is cleared; keep showing the last one.
	const [shown, setShown] = useState(subject);
	if (subject !== undefined && subject !== shown) setShown(subject);
	return (
		<Modal
			centered
			onClose={onCancel}
			opened={subject !== undefined}
			title={`Delete ${shown}?`}
		>
			<Text size="sm">
				The scanner stops calculating {shown}, and charts and tables no longer
				show it.
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
