import {
	createTheme,
	DEFAULT_THEME,
	mergeMantineTheme,
	NumberInput,
	Tooltip,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";

export const theme = createTheme({
	defaultRadius: "md",
	fontFamily:
		"Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
	primaryColor: "teal",
	components: {
		NumberInput: NumberInput.extend({ defaultProps: { hideControls: true } }),
		// Mantine inverts tooltip colors in the dark scheme (light gray, black
		// text); use the dark surface instead so tooltips match the app.
		Tooltip: Tooltip.extend({
			vars: (theme) => {
				const { colors } = themeToVars(theme);
				return {
					tooltip: {
						"--tooltip-bg": colors.dark[5],
						"--tooltip-color": colors.dark[0],
					},
				};
			},
		}),
	},
});

// The complete app theme, for code that needs its values outside React, such
// as chart colors drawn on a canvas.
export const resolvedTheme = mergeMantineTheme(DEFAULT_THEME, theme);
