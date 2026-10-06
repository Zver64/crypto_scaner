import { Box, Flex, type MantineSpacing } from "@mantine/core";
import type { ReactNode } from "react";
import { usePageViewport } from "@/app/page-height";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";

const sidebarWidth = 380;
const scrollColumn = {
	display: "flex",
	flexDirection: "column",
	overflowY: "auto",
} as const;

interface SidebarLayoutProps {
	children: ReactNode;
	gap: MantineSpacing;
	sidebar: ReactNode;
	sidebarPosition: "end" | "start";
}

// One column on smaller screens, with the sidebar before or after the main
// content. On wide screens the sidebar sits beside the content at a fixed
// width, the layout fits the page viewport, or the free height of a flex column
// parent, and the sidebar and the content scroll on their own.
export function SidebarLayout({
	children,
	gap,
	sidebar,
	sidebarPosition,
}: SidebarLayoutProps) {
	const viewport = usePageViewport();
	const wide = useWideLayout();
	const aside = (
		<Box
			flex={wide ? "none" : undefined}
			style={wide ? scrollColumn : undefined}
			w={wide ? sidebarWidth : "100%"}
		>
			{sidebar}
		</Box>
	);
	return (
		<Flex
			direction={wide ? "row" : "column"}
			gap={gap}
			h={wide ? viewport.height : undefined}
			// In a flex column parent the free height wins over the page height.
			style={wide ? { flex: 1, minHeight: 0 } : undefined}
		>
			{sidebarPosition === "start" ? aside : null}
			<Box
				flex={wide ? 1 : undefined}
				miw={0}
				// A flex column, so content can fill the height.
				style={wide ? scrollColumn : undefined}
			>
				{children}
			</Box>
			{sidebarPosition === "end" ? aside : null}
		</Flex>
	);
}
