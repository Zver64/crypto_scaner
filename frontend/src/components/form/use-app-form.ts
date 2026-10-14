import { createFormHook } from "@tanstack/react-form";

import { fieldContext, formContext } from "@/components/form/form-context";
import { MultiSelectField } from "@/components/form/multi-select-field";
import { NativeSelectField } from "@/components/form/native-select-field";
import { NumberField } from "@/components/form/number-field";
import { SelectField } from "@/components/form/select-field";
import { TagsField } from "@/components/form/tags-field";
import { TextField } from "@/components/form/text-field";
import { TextareaField } from "@/components/form/textarea-field";

// The application's forms, with Mantine inputs bound to their fields.
export const { useAppForm, withFieldGroup, withForm } = createFormHook({
	fieldComponents: {
		MultiSelectField,
		NativeSelectField,
		NumberField,
		SelectField,
		TagsField,
		TextareaField,
		TextField,
	},
	fieldContext,
	formComponents: {},
	formContext,
});
