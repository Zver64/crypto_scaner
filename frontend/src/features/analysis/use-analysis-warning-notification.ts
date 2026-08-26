import { notifications } from "@mantine/notifications";
import { useEffect } from "react";
import type { Warning } from "../../api/client";

export function useAnalysisWarningNotification(
	warnings: readonly Warning[] | undefined,
	title: string,
) {
	useEffect(() => {
		const shown = new Set<string>();
		for (const warning of warnings ?? []) {
			const key = `${warning.code}:${warning.message}`;
			if (shown.has(key)) {
				continue;
			}
			shown.add(key);
			notifications.show({
				autoClose: 8000,
				color: "yellow",
				message: warning.message,
				title,
			});
		}
	}, [title, warnings]);
}
