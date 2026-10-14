import { Modal } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import type {
	StrategyDraft,
	StrategyFormValues,
	StrategyVariableSets,
} from "@/features/strategy-settings/types";
import { strategyFormTitle } from "@/features/strategy-settings/utils";

interface StrategyFormProps {
	// The strategy being edited or created; the form is open while it is set.
	draft: StrategyDraft | undefined;
	form: UseFormReturnType<StrategyFormValues>;
	isSaving: boolean;
	onCancel(): void;
	onSubmit(): void;
	variables: StrategyVariableSets;
}

export function StrategyForm({
	draft,
	form,
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
			title={draft ? strategyFormTitle(draft) : undefined}
		>
			{draft ? (
				<StrategyFormContent
					draft={draft}
					form={form}
					isSaving={isSaving}
					key={draft.revision}
					onCancel={onCancel}
					onSubmit={onSubmit}
					variables={variables}
				/>
			) : null}
		</Modal>
	);
}
