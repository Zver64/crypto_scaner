import {
	Box,
	Button,
	Group,
	Input,
	NumberInput,
	type NumberInputProps,
	useMantineTheme,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { useId } from "react";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

interface NumberFieldPreset {
	label: string;
	value: number;
}

interface NumberFieldProps extends Omit<
	NumberInputProps,
	| "error"
	| "id"
	| "name"
	| "onBlur"
	| "onChange"
	| "onValueChange"
	| "value"
	| "valueIsNumericString"
> {
	// Holds the typed numeric string, never converted to a JavaScript number.
	asString?: boolean;
	// Values chosen with one tap, shown as buttons beside the input.
	presets?: readonly NumberFieldPreset[];
}

// Holds what NumberInput emits: a number, or a string while it is empty or
// being typed; with asString, always the string.
export function NumberField({
	asString,
	description,
	label,
	presets,
	required,
	withAsterisk,
	...props
}: NumberFieldProps) {
	const field = useFieldContext<number | string>();
	const id = useId();
	const { spacing } = themeToVars(useMantineTheme());
	const input: NumberInputProps = {
		...props,
		error: fieldError(field.state.meta.errors),
		id,
		name: field.name,
		onBlur: field.handleBlur,
		value: field.state.value,
		...(asString
			? {
					// Blur trimming would go through onChange, which is unused here.
					onValueChange: ({ value }, { source }) => {
						if (source !== "prop") field.handleChange(value);
					},
					trimLeadingZeroesOnBlur: false,
					valueIsNumericString: true,
				}
			: { onChange: field.handleChange }),
	};
	const wrapper = { description, label, required, withAsterisk };
	if (!presets?.length) return <NumberInput {...input} {...wrapper} />;
	// The label and description stand above the row, so the buttons line up
	// with the input. The row keeps the gap Mantine leaves below a label.
	return (
		<Input.Wrapper {...wrapper} id={id}>
			<Group
				align="flex-start"
				gap="xs"
				mt={`calc(${spacing.xs} / 2)`}
				wrap="nowrap"
			>
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
							// The input's size, so both are equally tall.
							size="sm"
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
