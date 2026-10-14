import { NativeSelect, type NativeSelectProps } from "@mantine/core";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

type NativeSelectFieldProps = Omit<
	NativeSelectProps,
	"error" | "name" | "onBlur" | "onChange" | "value"
>;

export function NativeSelectField(props: NativeSelectFieldProps) {
	const field = useFieldContext<string>();
	return (
		<NativeSelect
			{...props}
			error={fieldError(field.state.meta.errors)}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={(event) => field.handleChange(event.currentTarget.value)}
			value={field.state.value}
		/>
	);
}
