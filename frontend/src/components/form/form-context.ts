import { createFormHookContexts } from "@tanstack/react-form";

// The contexts the bound field and form components read their field and form
// from.
export const { fieldContext, formContext, useFieldContext, useFormContext } =
	createFormHookContexts();
