import { MultiSelect, type MultiSelectProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type MultiSelectFieldProps = Omit<
	MultiSelectProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

export function MultiSelectField(props: MultiSelectFieldProps) {
	const field = useFieldContext<string[]>();
	return (
		<MultiSelect
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={field.handleChange}
			value={field.state.value}
		/>
	);
}
