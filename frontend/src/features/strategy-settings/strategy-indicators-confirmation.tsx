import type { StrategyMissingIndicator } from "@/api/generated/models";
import { ConfirmationDialog } from "@/components/confirmation-dialog";

interface StrategyIndicatorsConfirmationProps {
	isPending: boolean;
	// The indicators to add before the strategy opens; undefined closes the
	// dialog.
	missing: StrategyMissingIndicator[] | undefined;
	onCancel(): void;
	onConfirm(): void;
}

// The builder shows only configured indicators, so a strategy that reads
// others opens once the administrator agrees to add them.
export function StrategyIndicatorsConfirmation({
	isPending,
	missing,
	onCancel,
	onConfirm,
}: StrategyIndicatorsConfirmationProps) {
	return (
		<ConfirmationDialog
			confirmColor="blue"
			confirmLabel="Add and edit"
			content={
				missing === undefined
					? undefined
					: {
							title: `Add ${missing.length} missing ${missing.length === 1 ? "indicator" : "indicators"}?`,
							description: `The strategy reads ${missing.map(({ title }) => title).join(", ")}, which ${missing.length === 1 ? "is" : "are"} not configured. Editing adds ${missing.length === 1 ? "it" : "them"} to the configured indicators.`,
						}
			}
			isPending={isPending}
			onCancel={onCancel}
			onConfirm={onConfirm}
		/>
	);
}
