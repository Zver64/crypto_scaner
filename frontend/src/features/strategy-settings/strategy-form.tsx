import { Modal } from "@mantine/core";

import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import type {
	StrategyDraft,
	StrategyFormApi,
	StrategyVariableSets,
} from "@/features/strategy-settings/types";
import { strategyFormTitle } from "@/features/strategy-settings/utils";

interface StrategyFormProps {
	// The strategy being edited or created; the form is open while it is set.
	draft: StrategyDraft | undefined;
	form: StrategyFormApi;
	isSaving: boolean;
	onCancel(): void;
	variables: StrategyVariableSets;
}

export function StrategyForm({
	draft,
	form,
	isSaving,
	onCancel,
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
					variables={variables}
				/>
			) : null}
		</Modal>
	);
}
