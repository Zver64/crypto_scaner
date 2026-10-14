import {
	parseThemeColor,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";

// Resolves the Mantine theme colors of indicator lines, such as "blue.5", for
// the current color scheme where they are drawn.
export function useThemeColor(): (color: string) => string {
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	return (color) => parseThemeColor({ color, colorScheme, theme }).value;
}
