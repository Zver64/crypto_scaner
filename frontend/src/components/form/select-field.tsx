import { Select, type SelectProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type SelectFieldProps = Omit<
	SelectProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

export function SelectField(props: SelectFieldProps) {
	const field = useFieldContext<string | null>();
	return (
		<Select
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={field.handleChange}
			value={field.state.value}
		/>
	);
}
