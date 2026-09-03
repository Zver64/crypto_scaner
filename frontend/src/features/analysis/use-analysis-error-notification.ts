import { notifications } from "@mantine/notifications";
import { useEffect } from "react";
import { ApiError } from "@/api/client";

export function useAnalysisErrorNotification(
	error: Error | null,
	title: string,
) {
	useEffect(() => {
		if (!error) {
			return;
		}

		notifications.show({
			autoClose: 5000,
			color: "red",
			message:
				error instanceof ApiError
					? error.message
					: "An unexpected error occurred. Please try again.",
			title,
		});
	}, [error, title]);
}
