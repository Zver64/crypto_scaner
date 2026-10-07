import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface ApiTokenRemovalConfirmationProps {
	isPending: boolean;
	name: string | undefined;
	onCancel(): void;
	onConfirm(): void;
}

export function ApiTokenRemovalConfirmation({
	isPending,
	name,
	onCancel,
	onConfirm,
}: ApiTokenRemovalConfirmationProps) {
	return (
		<ConfirmationDialog
			confirmLabel="Delete"
			content={
				name === undefined
					? undefined
					: {
							title: `Delete ${name}?`,
							description: "Requests with this token are rejected from now on.",
						}
			}
			isPending={isPending}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
