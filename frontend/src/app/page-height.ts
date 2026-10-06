import { useMantineTheme } from "@mantine/core";

export const headerContentHeight = "3.25rem";

// The viewport area left for a page below the app header, inside the
// AppShell padding used from the sm breakpoint: its height and its top
// offset, for content that sticks below the header.
export function usePageViewport() {
	const theme = useMantineTheme();
	return {
		height: `calc(100dvh - ${headerContentHeight} - 2 * ${theme.spacing.sm})`,
		top: `calc(${headerContentHeight} + ${theme.spacing.sm})`,
	};
}
