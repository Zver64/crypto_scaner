import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface UserRemovalConfirmationProps {
	isPending: boolean;
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
	return (
		<ConfirmationDialog
			confirmLabel="Delete"
			content={
				name === undefined
					? undefined
					: {
							title: `Delete ${name}?`,
							description: `${name} loses access to the scanner, and their favorites and alerts are deleted. The bot can add them again later.`,
						}
			}
			isPending={isPending}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
