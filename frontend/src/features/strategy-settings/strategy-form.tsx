import { Modal } from "@mantine/core";
import type { StrategyUpdate, StrategyVariable } from "@/api/generated/models";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import type { StrategyDraft } from "@/features/strategy-settings/types";

interface StrategyFormProps {
	// The strategy being edited or created; the form is open while it is set.
	draft: StrategyDraft | undefined;
	isSaving: boolean;
	onCancel(): void;
	onSubmit(input: StrategyUpdate): void;
	variables: readonly StrategyVariable[];
}

export function StrategyForm({
	draft,
	isSaving,
	onCancel,
	onSubmit,
	variables,
}: StrategyFormProps) {
	return (
		<Modal
			fullScreen
			onClose={onCancel}
			opened={draft !== undefined}
			title={draft?.id === undefined ? "New strategy" : "Edit strategy"}
		>
			{draft ? (
				<StrategyFormContent
					draft={draft}
					isSaving={isSaving}
					key={draft.id ?? "new"}
					onCancel={onCancel}
					onSubmit={onSubmit}
					variables={variables}
				/>
			) : null}
		</Modal>
	);
}
