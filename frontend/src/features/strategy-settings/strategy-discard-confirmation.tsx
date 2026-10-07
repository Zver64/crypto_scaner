import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface StrategyDiscardConfirmationProps {
	// Whether another strategy waits to replace the one with unsaved changes.
	opened: boolean;
	onCancel(): void;
	onConfirm(): void;
}

export function StrategyDiscardConfirmation({
	opened,
	onCancel,
	onConfirm,
}: StrategyDiscardConfirmationProps) {
	return (
		<ConfirmationDialog
			confirmLabel="Discard"
			content={
				opened
					? {
							title: "Discard changes?",
							description: "The open strategy has unsaved changes.",
						}
					: undefined
			}
			isPending={false}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
