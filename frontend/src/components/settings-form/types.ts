import type { FormEventHandler, ReactNode } from "react";
import type { NumberInputField } from "@/components/number-input-fieldset";

export interface SettingsFormGroup {
	id: string;
	title: string;
	inputs: readonly NumberInputField[];
}

export interface SettingsFormProps {
	groups: readonly SettingsFormGroup[];
	disabled?: boolean;
	onSubmit: FormEventHandler<HTMLFormElement>;
	submitLabel: ReactNode;
}
