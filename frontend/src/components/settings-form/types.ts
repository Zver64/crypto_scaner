import type { SimpleGridProps } from "@mantine/core";
import type { FormEventHandler, ReactNode } from "react";
import type { NumberInputField } from "@/components/number-input-fieldset";

export interface SettingsFormGroup {
	id: string;
	title: string;
	inputs: readonly NumberInputField[];
}

export interface SettingsFormProps {
	// Grid columns of the groups; one per row by default.
	columns?: SimpleGridProps["cols"];
	groups: readonly SettingsFormGroup[];
	disabled?: boolean;
	loading?: boolean;
	onSubmit: FormEventHandler<HTMLFormElement>;
	submitLabel: ReactNode;
}
