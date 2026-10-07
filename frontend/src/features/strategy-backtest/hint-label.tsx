import { Text, Tooltip } from "@mantine/core";
import type { ReactNode } from "react";

interface HintLabelProps {
	children: ReactNode;
	hint: string;
}

// Text explained by a tooltip on hover, focus or tap; the dotted underline
// shows that it has one.
export function HintLabel({ children, hint }: HintLabelProps) {
	return (
		<Tooltip
			events={{ focus: true, hover: true, touch: true }}
			label={hint}
			maw={280}
			multiline
		>
			<Text
				component="span"
				inherit
				style={{ cursor: "help", textDecoration: "underline dotted" }}
			>
				{children}
			</Text>
		</Tooltip>
	);
}
