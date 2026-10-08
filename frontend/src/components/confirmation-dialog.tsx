import { Button, Group, type MantineColor, Modal, Text } from "@mantine/core";
import { useState } from "react";

interface ConfirmationDialogProps {
	// Red by default, for actions that remove something.
	confirmColor?: MantineColor;
	confirmLabel: string;
	// Undefined closes the dialog; its last content stays during the fade-out.
	content: { title: string; description: string } | undefined;
	isPending: boolean;
	onCancel(): void;
	onConfirm(): void;
}

export function ConfirmationDialog({
	confirmColor = "red",
	confirmLabel,
	content,
	isPending,
	onCancel,
	onConfirm,
}: ConfirmationDialogProps) {
	const [shown, setShown] = useState(content);
	if (
		content !== undefined &&
		(content.title !== shown?.title ||
			content.description !== shown?.description)
	) {
		setShown(content);
	}
	return (
		<Modal
			centered
			onClose={onCancel}
			opened={content !== undefined}
			title={shown?.title}
		>
			<Text size="sm">{shown?.description}</Text>
			<Group justify="flex-end" mt="md">
				<Button disabled={isPending} onClick={onCancel} variant="default">
					Cancel
				</Button>
				<Button color={confirmColor} loading={isPending} onClick={onConfirm}>
					{confirmLabel}
				</Button>
			</Group>
		</Modal>
	);
}
