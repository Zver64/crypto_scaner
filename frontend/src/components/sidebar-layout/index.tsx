import { Box, Flex, type MantineSpacing } from "@mantine/core";
import type { ReactNode } from "react";
import { usePageViewport } from "@/app/page-height";
import { useSidebarLayoutActive } from "@/components/sidebar-layout/use-sidebar-layout-active";

const sidebarWidth = 380;
const scrollStyle = { overflowY: "auto" } as const;

interface SidebarLayoutProps {
	children: ReactNode;
	// How the side-by-side layout uses the viewport: "fill" fits the page to
	// it, so the sidebar and the main content scroll on their own; "sticky"
	// keeps the sidebar in view while the page scrolls.
	desktop: "fill" | "sticky";
	gap: MantineSpacing;
	sidebar: ReactNode;
	sidebarPosition: "end" | "start";
}

// One column on smaller screens, with the sidebar before or after the main
// content; from the sidebar layout breakpoint the sidebar sits beside it at a
// fixed width.
export function SidebarLayout({
	children,
	desktop,
	gap,
	sidebar,
	sidebarPosition,
}: SidebarLayoutProps) {
	const viewport = usePageViewport();
	const active = useSidebarLayoutActive();
	const fill = active && desktop === "fill";
	const sticky = active && desktop === "sticky";
	const aside = (
		<Box
			flex={active ? "none" : undefined}
			mah={sticky ? viewport.height : undefined}
			pos={sticky ? "sticky" : undefined}
			style={
				sticky
					? { ...scrollStyle, alignSelf: "flex-start" }
					: fill
						? scrollStyle
						: undefined
			}
			top={sticky ? viewport.top : undefined}
			w={active ? sidebarWidth : "100%"}
		>
			{sidebar}
		</Box>
	);
	return (
		<Flex
			direction={active ? "row" : "column"}
			gap={gap}
			h={fill ? viewport.height : undefined}
		>
			{sidebarPosition === "start" ? aside : null}
			<Box
				flex={active ? 1 : undefined}
				miw={0}
				style={
					fill
						? { ...scrollStyle, display: "flex", flexDirection: "column" }
						: undefined
				}
			>
				{children}
			</Box>
			{sidebarPosition === "end" ? aside : null}
		</Flex>
	);
}
