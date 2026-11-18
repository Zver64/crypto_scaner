import type { ComponentProps } from "react";

import { useFieldContext } from "@/components/form/form-context";
import { SliderField } from "@/components/slider-field";

type ScaleSliderFieldProps = Omit<
	ComponentProps<typeof SliderField>,
	"name" | "onBlur" | "onChange" | "value"
>;

// A number chosen on a slider that shows its value beside the label and
// labels its scale below the track.
export function ScaleSliderField(props: ScaleSliderFieldProps) {
	const field = useFieldContext<number>();
	return (
		<SliderField
			{...props}
			name={field.name}
			onBlur={field.handleBlur}
			onChange={field.handleChange}
			value={field.state.value}
		/>
	);
}
