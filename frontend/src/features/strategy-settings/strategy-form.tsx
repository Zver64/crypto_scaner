import { Modal } from "@mantine/core";
import type { StrategyUpdate, StrategyVariable } from "@/api/generated/models";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import { strategyFormTitle } from "@/features/strategy-settings/utils";

interface StrategyFormProps {
	// The strategy being edited or created; the form is open while it is set.
	draft: StrategyDraft | undefined;
	isSaving: boolean;
	onCancel(): void;
	onDirtyChange(dirty: boolean): void;
	onSubmit(input: StrategyUpdate): void;
	variables: readonly StrategyVariable[];
}

export function StrategyForm({
	draft,
	isSaving,
	onCancel,
	onDirtyChange,
	onSubmit,
	variables,
}: StrategyFormProps) {
	return (
		<Modal
			fullScreen
			onClose={onCancel}
			opened={draft !== undefined}
			title={draft ? strategyFormTitle(draft) : undefined}
		>
			{draft ? (
				<StrategyFormContent
					draft={draft}
					isSaving={isSaving}
					key={draft.revision}
					onCancel={onCancel}
					onDirtyChange={onDirtyChange}
					onSubmit={onSubmit}
					variables={variables}
				/>
			) : null}
		</Modal>
	);
}
