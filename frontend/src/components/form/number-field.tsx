import {
	Box,
	Button,
	Group,
	Input,
	NumberInput,
	type NumberInputProps,
} from "@mantine/core";
import { useId } from "react";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

interface NumberFieldPreset {
	label: string;
	value: number;
}

interface NumberFieldProps extends Omit<
	NumberInputProps,
	"error" | "id" | "name" | "onBlur" | "onChange" | "value"
> {
	// Values chosen with one tap, shown as buttons beside the input.
	presets?: readonly NumberFieldPreset[];
}

// Holds what NumberInput emits: a number, or a string while it is empty or
// being typed.
export function NumberField({
	description,
	label,
	presets,
	required,
	withAsterisk,
	...props
}: NumberFieldProps) {
	const field = useFieldContext<number | string>();
	const id = useId();
	const input = {
		...props,
		error: fieldError(field.state.meta.errors),
		id,
		name: field.name,
		onBlur: field.handleBlur,
		onChange: field.handleChange,
		value: field.state.value,
	};
	const wrapper = { description, label, required, withAsterisk };
	if (!presets?.length) return <NumberInput {...input} {...wrapper} />;
	// The label and description stand above the row, so the buttons line up
	// with the input.
	return (
		<Input.Wrapper {...wrapper} id={id}>
			<Group align="flex-start" gap="xs" wrap="nowrap">
				<Box flex={1}>
					<NumberInput {...input} />
				</Box>
				<Group gap="xs" justify="center">
					{presets.map((preset) => (
						<Button
							disabled={props.disabled}
							key={preset.value}
							onClick={(event) => {
								field.handleChange(preset.value);
								event.currentTarget.blur();
							}}
							size="xs"
							variant="default"
						>
							{preset.label}
						</Button>
					))}
				</Group>
			</Group>
		</Input.Wrapper>
	);
}
