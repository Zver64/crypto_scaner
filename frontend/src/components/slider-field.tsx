import { Box, Slider, type SliderProps, Stack, Text } from "@mantine/core";
import type { ReactNode } from "react";

interface SliderScaleLabel {
	label: ReactNode;
	position: number;
}

interface SliderFieldProps
	extends Omit<
		SliderProps,
		"label" | "thumbLabel" | "thumbValueText" | "value"
	> {
	formatValue: (value: number) => string;
	label: string;
	scaleLabels: readonly SliderScaleLabel[];
	value: number;
}

export function SliderField({
	formatValue,
	label,
	scaleLabels,
	value,
	...sliderProps
}: SliderFieldProps) {
	return (
		<Stack gap={4}>
			<Text fw={500} size="sm">
				{label}: {formatValue(value)}
			</Text>
			<Slider
				{...sliderProps}
				label={formatValue}
				thumbLabel={label}
				thumbValueText={formatValue}
				value={value}
			/>
			<Box h={18} pos="relative">
				{scaleLabels.map(({ label: scaleLabel, position }) => (
					<Text
						c="dimmed"
						key={position}
						pos="absolute"
						size="xs"
						style={{
							left: `${position}%`,
							transform:
								position === 0
									? undefined
									: position === 100
										? "translateX(-100%)"
										: "translateX(-50%)",
							whiteSpace: "nowrap",
						}}
					>
						{scaleLabel}
					</Text>
				))}
			</Box>
		</Stack>
	);
}
