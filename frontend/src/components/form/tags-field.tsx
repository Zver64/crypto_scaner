import { TagsInput, type TagsInputProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type TagsFieldProps = Omit<
	TagsInputProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

export function TagsField(props: TagsFieldProps) {
	const field = useFieldContext<string[]>();
	return (
		<TagsInput
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={field.handleChange}
			value={field.state.value}
		/>
	);
}
