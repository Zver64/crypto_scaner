import { createFormHook } from "@tanstack/react-form";

import { fieldContext, formContext } from "@/components/form/form-context";
import { MultiSelectField } from "@/components/form/multi-select-field";
import { NativeSelectField } from "@/components/form/native-select-field";
import { NumberField } from "@/components/form/number-field";
import { SelectField } from "@/components/form/select-field";
import { SliderField } from "@/components/form/slider-field";
import { SubmitButton } from "@/components/form/submit-button";
import { TagsField } from "@/components/form/tags-field";
import { TextField } from "@/components/form/text-field";
import { TextareaField } from "@/components/form/textarea-field";

// The only entry point to forms: create a form with useAppForm, share parts
// of one with withForm or withFieldGroup, and render the bound inputs as
// field.TextField and the like inside form.AppField, and the submit button
// as form.SubmitButton inside form.AppForm. A feature's own input inside
// form.Field shows its error with fieldError(field.state.meta.errors).
export { fieldError } from "@/components/form/utils";
export const { useAppForm, withFieldGroup, withForm } = createFormHook({
	fieldComponents: {
		MultiSelectField,
		NativeSelectField,
		NumberField,
		SelectField,
		SliderField,
		TagsField,
		TextareaField,
		TextField,
	},
	fieldContext,
	formComponents: { SubmitButton },
	formContext,
});
