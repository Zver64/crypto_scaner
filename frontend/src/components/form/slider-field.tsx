import {
	Input,
	Slider,
	type SliderProps,
	useMantineTheme,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import type { ReactNode } from "react";

import { useFieldContext } from "@/components/form/form-context";
import { fieldError } from "@/components/form/utils";

interface SliderFieldProps extends Omit<
	SliderProps,
	"label" | "name" | "onBlur" | "onChange" | "value"
> {
	label: ReactNode;
}

// A number chosen on a slider; its marks are labeled below the track. Holds
// what a number input would, so a string value is read as a number. The
// slider keeps the gap Mantine leaves below a label.
export function SliderField({ label, ...props }: SliderFieldProps) {
	const field = useFieldContext<number | string>();
	const { spacing } = themeToVars(useMantineTheme());
	return (
		<Input.Wrapper error={fieldError(field.state.meta.errors)} label={label}>
			<Slider
				{...props}
				mb="lg"
				mt={`calc(${spacing.xs} / 2)`}
				name={field.name}
				onBlur={field.handleBlur}
				onChange={field.handleChange}
				value={Number(field.state.value)}
			/>
		</Input.Wrapper>
	);
}
