import { Code, Loader, SimpleGrid, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import type { ErrorType } from "@/api/fetch";
import {
	getListApiTokensQueryKey,
	useCreateApiToken,
	useDeleteApiToken,
	useListApiTokens,
} from "@/api/generated/api";
import type { ApiToken, ErrorResponse } from "@/api/generated/models";
import { ApiTokenCreate } from "@/features/api-token-settings/api-token-create";
import { ApiTokenRemovalConfirmation } from "@/features/api-token-settings/api-token-removal-confirmation";
import { ApiTokenRow } from "@/features/api-token-settings/api-token-row";
import { IssuedApiToken } from "@/features/api-token-settings/issued-api-token";
import { apiTokenErrorMessage } from "@/features/api-token-settings/utils";

// Tokens of the administrator for the scanner CLI and other API clients.
export function ApiTokenSettings() {
	const queryClient = useQueryClient();
	const [removing, setRemoving] = useState<ApiToken>();
	const tokens = useListApiTokens({
		query: { retry: false, select: (response) => response.data.items },
	});
	const refresh = () =>
		queryClient.invalidateQueries({ queryKey: getListApiTokensQueryKey() });
	const failed = (title: string) => (error: ErrorType<ErrorResponse>) => {
		notifications.show({
			color: "red",
			message: apiTokenErrorMessage(error),
			title,
		});
	};
	// The plaintext token lives only in this mutation's result: it is dropped
	// from the mutation cache as soon as the dialog is dismissed.
	const createMutation = useCreateApiToken({
		mutation: {
			gcTime: 0,
			onError: failed("Token was not created"),
			onSettled: () => {
				// Showing the one-time secret must not wait for the list refetch.
				void refresh();
			},
		},
	});
	const deleteMutation = useDeleteApiToken({
		mutation: {
			onError: failed("Token was not deleted"),
			onSettled: () => {
				setRemoving(undefined);
				return refresh();
			},
		},
	});

	return (
		<Stack gap="sm">
			{tokens.isPending ? (
				<Loader />
			) : tokens.isError ? (
				<Text c="red" size="sm">
					API tokens could not be loaded.
				</Text>
			) : (
				<>
					<Text c="dimmed" size="sm">
						API tokens sign in the <Code>scanner</Code> CLI with administrator
						rights. Delete a token to revoke it.
					</Text>
					<ApiTokenCreate
						isPending={createMutation.isPending}
						onCreate={(name, onCreated) =>
							createMutation.mutate(
								{ data: { name } },
								{ onSuccess: onCreated },
							)
						}
					/>
					{tokens.data.length === 0 ? (
						<Text c="dimmed" size="sm">
							No API tokens yet.
						</Text>
					) : (
						<SimpleGrid cols={{ base: 1, sm: 2, xl: 3 }} spacing="xs">
							{tokens.data.map((token) => (
								<ApiTokenRow
									disabled={deleteMutation.isPending}
									key={token.id}
									onDelete={() => setRemoving(token)}
									token={token}
								/>
							))}
						</SimpleGrid>
					)}
				</>
			)}
			<IssuedApiToken
				onDismiss={createMutation.reset}
				token={createMutation.data?.data.token}
			/>
			<ApiTokenRemovalConfirmation
				isPending={deleteMutation.isPending}
				name={removing?.name}
				onCancel={() => setRemoving(undefined)}
				onConfirm={() => {
					if (removing) deleteMutation.mutate({ tokenId: removing.id });
				}}
			/>
		</Stack>
	);
}
