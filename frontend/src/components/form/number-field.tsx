import { NumberInput, type NumberInputProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type NumberFieldProps = Omit<
	NumberInputProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

// Holds what NumberInput emits: a number, or a string while it is empty or
// being typed.
export function NumberField(props: NumberFieldProps) {
	const field = useFieldContext<number | string>();
	return (
		<NumberInput
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={field.handleChange}
			value={field.state.value}
		/>
	);
}
