import { Loader, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
	getListUsersQueryKey,
	useDeleteUser,
	useListUsers,
} from "@/api/generated/api";
import type { User } from "@/api/generated/models";
import { telegramRequestOptions } from "@/app/telegram";
import { UserRemovalConfirmation } from "@/features/user-settings/user-removal-confirmation";
import { UserRow } from "@/features/user-settings/user-row";
import {
	deleteUserErrorMessage,
	formatUserName,
} from "@/features/user-settings/utils";

export function UserSettings() {
	const queryClient = useQueryClient();
	const [removing, setRemoving] = useState<User>();
	const users = useListUsers({
		fetch: telegramRequestOptions(),
		query: { retry: false, select: (response) => response.data.items },
	});
	const deleteMutation = useDeleteUser({
		fetch: telegramRequestOptions(),
		mutation: {
			onError: (error) => {
				notifications.show({
					color: "red",
					message: deleteUserErrorMessage(error),
					title: "User was not deleted",
				});
			},
			onSettled: () => {
				setRemoving(undefined);
				return queryClient.invalidateQueries({
					queryKey: getListUsersQueryKey(),
				});
			},
		},
	});

	if (users.isPending) {
		return <Loader aria-label="Loading users" />;
	}
	if (users.isError) {
		return (
			<Text c="red" size="sm">
				Users could not be loaded.
			</Text>
		);
	}
	return (
		<Stack gap="xs">
			<Text c="dimmed" size="sm">
				Add users from the chat with the bot.
			</Text>
			{users.data.map((user) => (
				<UserRow
					disabled={deleteMutation.isPending}
					key={user.telegram_id}
					onDelete={() => setRemoving(user)}
					user={user}
				/>
			))}
			<UserRemovalConfirmation
				isPending={deleteMutation.isPending}
				name={removing ? formatUserName(removing) : undefined}
				onCancel={() => setRemoving(undefined)}
				onConfirm={() => {
					if (removing)
						deleteMutation.mutate({ telegramId: removing.telegram_id });
				}}
			/>
		</Stack>
	);
}
