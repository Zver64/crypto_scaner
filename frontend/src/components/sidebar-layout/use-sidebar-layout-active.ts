import { useMatches } from "@mantine/core";
import { sidebarLayoutBreakpoint } from "@/components/sidebar-layout/constants";

// Whether the sidebar sits beside the main content, for content that adapts
// to the side-by-side layout beyond what CSS breakpoints can express.
export function useSidebarLayoutActive() {
	return useMatches(
		{ base: false, [sidebarLayoutBreakpoint]: true },
		{ getInitialValueInEffect: false },
	);
}
