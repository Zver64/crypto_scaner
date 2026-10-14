import { Textarea, type TextareaProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type TextareaFieldProps = Omit<
	TextareaProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

export function TextareaField(props: TextareaFieldProps) {
	const field = useFieldContext<string>();
	return (
		<Textarea
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={(event) => field.handleChange(event.currentTarget.value)}
			value={field.state.value}
		/>
	);
}
