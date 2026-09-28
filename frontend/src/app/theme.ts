import {
	createTheme,
	DEFAULT_THEME,
	mergeMantineTheme,
	NumberInput,
} from "@mantine/core";

export const theme = createTheme({
	defaultRadius: "md",
	fontFamily:
		"Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
	primaryColor: "teal",
	components: {
		NumberInput: NumberInput.extend({ defaultProps: { hideControls: true } }),
	},
});

// The complete app theme, for code that needs its values outside React, such
// as chart colors drawn on a canvas.
export const resolvedTheme = mergeMantineTheme(DEFAULT_THEME, theme);
