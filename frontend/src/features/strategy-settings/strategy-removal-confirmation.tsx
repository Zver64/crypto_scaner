import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface StrategyRemovalConfirmationProps {
	isPending: boolean;
	name: string | undefined;
	// The strategy or signal, as its page calls it.
	noun: string;
	onCancel(): void;
	onConfirm(): void;
}

export function StrategyRemovalConfirmation({
	isPending,
	name,
	noun,
	onCancel,
	onConfirm,
}: StrategyRemovalConfirmationProps) {
	return (
		<ConfirmationDialog
			confirmLabel="Delete"
			content={
				name === undefined
					? undefined
					: {
							title: `Delete ${name}?`,
							description: `No more alerts will be sent for this ${noun}.`,
						}
			}
			isPending={isPending}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
