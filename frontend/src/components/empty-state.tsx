import { Paper, Text } from "@mantine/core";
import type { ReactNode } from "react";

interface EmptyStateProps {
	// Actions below the message.
	children?: ReactNode;
	description?: ReactNode;
	failed?: boolean;
	// Fills the free height of a flex column parent, centering the message.
	fillHeight?: boolean;
	title: ReactNode;
}

// A card that stands in for content that is missing, failed or not chosen yet.
export function EmptyState({
	children,
	description,
	failed = false,
	fillHeight = false,
	title,
}: EmptyStateProps) {
	return (
		<Paper
			display={fillHeight ? "flex" : undefined}
			flex={fillHeight ? 1 : undefined}
			p="xl"
			style={
				fillHeight
					? { flexDirection: "column", justifyContent: "center" }
					: undefined
			}
			ta="center"
		>
			<Text c={failed ? "red" : undefined} fw={600}>
				{title}
			</Text>
			{description ? (
				<Text c="dimmed" mt={4} size="sm">
					{description}
				</Text>
			) : null}
			{children}
		</Paper>
	);
}
