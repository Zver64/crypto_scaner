import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface ScannerIndicatorRemovalConfirmationProps {
	isPending: boolean;
	subject: string | undefined;
	onCancel(): void;
	onConfirm(): void;
}

export function ScannerIndicatorRemovalConfirmation({
	isPending,
	subject,
	onCancel,
	onConfirm,
}: ScannerIndicatorRemovalConfirmationProps) {
	return (
		<ConfirmationDialog
			confirmLabel="Delete"
			content={
				subject === undefined
					? undefined
					: {
							title: `Delete ${subject}?`,
							description: `The scanner stops calculating ${subject}, and charts and tables no longer show it.`,
						}
			}
			isPending={isPending}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
