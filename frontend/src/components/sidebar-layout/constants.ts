import type { MantineBreakpoint } from "@mantine/core";

// The breakpoint from which the sidebar sits beside the main content.
export const sidebarLayoutBreakpoint = "lg" satisfies MantineBreakpoint;

// Grid columns for groups inside the sidebar: two per row on tablets, where
// the sidebar spans the page, and one beside the main content.
export const sidebarColumns = {
	base: 1,
	sm: 2,
	[sidebarLayoutBreakpoint]: 1,
};
