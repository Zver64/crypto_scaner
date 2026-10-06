import { Box, LoadingOverlay } from "@mantine/core";
import type { ReactNode } from "react";

interface RefreshingOverlayProps {
	children: ReactNode;
	// Fills the free height of a flex column parent and lays its content out
	// as a column over that height.
	fillHeight?: boolean;
	label: string;
	visible: boolean;
}

export function RefreshingOverlay({
	children,
	fillHeight = false,
	label,
	visible,
}: RefreshingOverlayProps) {
	return (
		<Box
			flex={fillHeight ? 1 : undefined}
			pos="relative"
			style={
				fillHeight ? { display: "flex", flexDirection: "column" } : undefined
			}
		>
			<LoadingOverlay
				loaderProps={{ "aria-label": label }}
				overlayProps={{ blur: 1 }}
				visible={visible}
				zIndex={10}
			/>
			{children}
		</Box>
	);
}
