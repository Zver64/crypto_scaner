import { Modal } from "@mantine/core";

import type { StrategyCatalog } from "@/api/generated/models";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import type {
	StrategyDraft,
	StrategyFormApi,
} from "@/features/strategy-settings/types";
import { strategyFormTitle } from "@/features/strategy-settings/utils";

interface StrategyFormProps {
	catalog: StrategyCatalog;
	// The strategy being edited or created; the form is open while it is set.
	draft: StrategyDraft | undefined;
	form: StrategyFormApi;
	isSaving: boolean;
	onCancel(): void;
}

export function StrategyForm({
	catalog,
	draft,
	form,
	isSaving,
	onCancel,
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
					catalog={catalog}
					draft={draft}
					form={form}
					isSaving={isSaving}
					key={draft.revision}
					onCancel={onCancel}
				/>
			) : null}
		</Modal>
	);
}
