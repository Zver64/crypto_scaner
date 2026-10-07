import { Box, type BoxProps, useMantineTheme } from "@mantine/core";
import type { ReactNode } from "react";
import { nestingBorder } from "@/features/strategy-settings/nesting-colors";

export interface NestingLineProps extends BoxProps {
	children: ReactNode;
	// How many groups and functions enclose the content, which picks the line
	// color.
	depth: number;
}

// Indents nested content behind a line in the color of its depth instead of
// a frame, so the content keeps the screen width.
export function NestingLine({ children, depth, ...props }: NestingLineProps) {
	return (
		<Box
			{...props}
			pl="sm"
			style={{
				borderLeftColor: nestingBorder(useMantineTheme(), depth),
				borderLeftStyle: "solid",
				borderLeftWidth: 2,
			}}
		>
			{children}
		</Box>
	);
}
