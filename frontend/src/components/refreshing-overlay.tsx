import { Box, LoadingOverlay } from "@mantine/core";
import type { ReactNode } from "react";

interface RefreshingOverlayProps {
	children: ReactNode;
	label: string;
	visible: boolean;
}

export function RefreshingOverlay({
	children,
	label,
	visible,
}: RefreshingOverlayProps) {
	return (
		<Box pos="relative">
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
