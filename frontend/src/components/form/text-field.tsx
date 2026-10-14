import { TextInput, type TextInputProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type TextFieldProps = Omit<
	TextInputProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

export function TextField(props: TextFieldProps) {
	const field = useFieldContext<string>();
	return (
		<TextInput
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={(event) => field.handleChange(event.currentTarget.value)}
			value={field.state.value}
		/>
	);
}
