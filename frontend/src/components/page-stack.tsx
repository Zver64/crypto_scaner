import { type MantineSpacing, Stack } from "@mantine/core";
import type { ReactNode } from "react";
import { usePageViewport } from "@/app/page-height";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";

interface PageStackProps {
	children: ReactNode;
	gap: MantineSpacing;
}

// A page column that fits the viewport on wide screens, so a sidebar layout
// below fixed blocks such as the page navigation fills the remaining height
// instead of making the page scroll.
export function PageStack({ children, gap }: PageStackProps) {
	const viewport = usePageViewport();
	const wide = useWideLayout();
	return (
		<Stack gap={gap} h={wide ? viewport.height : undefined}>
			{children}
		</Stack>
	);
}
