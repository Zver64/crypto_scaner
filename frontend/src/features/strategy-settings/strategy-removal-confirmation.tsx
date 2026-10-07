import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface StrategyRemovalConfirmationProps {
	isPending: boolean;
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
	return (
		<ConfirmationDialog
			confirmLabel="Delete"
			content={
				name === undefined
					? undefined
					: {
							title: `Delete ${name}?`,
							description: "No more alerts will be sent for this strategy.",
						}
			}
			isPending={isPending}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
