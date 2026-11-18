import { notifications } from "@mantine/notifications";

import type { APIErrorCode } from "@/api/generated/models";
import { type ApiError, describeQueryError } from "@/utils/api-error";

export type QueryErrorNotification = {
	id: string;
	title: string;
	// An error the screen shows in place instead of notifying.
	ignoredCode?: APIErrorCode;
	// Only a failed first load notifies; a failed refetch keeps the data quietly.
	initialLoadOnly?: boolean;
};

export function showQueryErrorNotification(
	{ id, title }: QueryErrorNotification,
	error: ApiError,
) {
	notifications.show({
		id,
		autoClose: 5000,
		color: "red",
		message: describeQueryError(error),
		title,
	});
}
