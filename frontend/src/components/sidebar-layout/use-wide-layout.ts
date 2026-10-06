import { useMatches } from "@mantine/core";
import { sidebarLayoutBreakpoint } from "@/components/sidebar-layout/constants";

// Whether the screen is wide enough for the side-by-side layout, for content
// that adapts to it beyond what CSS breakpoints can express.
export function useWideLayout() {
	return useMatches(
		{ base: false, [sidebarLayoutBreakpoint]: true },
		{ getInitialValueInEffect: false },
	);
}
