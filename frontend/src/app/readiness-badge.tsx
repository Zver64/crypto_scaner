import { Badge } from "@mantine/core";
import type { ReadinessStatus } from "@/app/types";

const readinessPresentation = {
	checking: { color: "yellow", label: "Checking" },
	ready: { color: "teal", label: "Ready" },
	unavailable: { color: "red", label: "Unavailable" },
} as const satisfies Record<ReadinessStatus, { color: string; label: string }>;

export function ReadinessBadge({ status }: { status: ReadinessStatus }) {
	const presentation = readinessPresentation[status];

	return (
		<Badge color={presentation.color} size="sm" variant="light">
			{presentation.label}
		</Badge>
	);
}
