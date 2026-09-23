import { Paper, Stack, Text, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useCallback, useEffect, useState } from "react";
import {
	getListPriceAlertsQueryKey,
	useCreatePriceAlert,
	useDeletePriceAlert,
	useListPriceAlerts,
	useUpdatePriceAlert,
} from "@/api/generated/api";
import { telegramRequestOptions } from "@/app/telegram";
import { useFavorites } from "@/features/favorites/favorites-provider";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";
import { scopedUserQueryKey } from "@/features/favorites/user-query-scope";
import { PriceAlertForm } from "@/features/price-alerts/price-alert-form";
import { PriceAlertList } from "@/features/price-alerts/price-alert-list";
import { normalizePriceTarget } from "@/features/price-alerts/target";

function mutationErrorMessage(error: unknown): string {
	const code = (error as { info?: { error?: { code?: unknown } } }).info?.error
		?.code;
	switch (code) {
		case "duplicate_target":
			return "An alert with this target already exists.";
		case "alert_limit":
			return "The maximum number of alerts for this instrument has been reached.";
		case "symbol_not_found":
			return "This instrument is no longer active.";
		case "alert_not_found":
			return "This alert no longer exists.";
		case "access_denied":
			return "Your Telegram account no longer has access.";
		case "unauthenticated":
			return "Telegram authorization has expired. Reopen the Mini App.";
		default:
			return "The price alert could not be saved.";
	}
}

interface PriceAlertsControllerProps {
	scope: string;
	symbol: string;
}

export function PriceAlertsController({
	scope,
	symbol,
}: PriceAlertsControllerProps) {
	const queryClient = useQueryClient();
	const { handleAccessError } = useFavorites();
	const query = useListPriceAlerts(symbol, {
		fetch: telegramRequestOptions(),
		query: {
			queryKey: scopedUserQueryKey(getListPriceAlertsQueryKey(symbol), scope),
			refetchInterval: 15_000,
			refetchOnMount: "always",
			retry: false,
			select: (response) => response.data,
		},
	});
	const [target, setTarget] = useState("");
	const [targetError, setTargetError] = useState<string>();
	const [editingID, setEditingID] = useState<number>();
	const resetEditor = useCallback(() => {
		setTarget("");
		setTargetError(undefined);
		setEditingID(undefined);
	}, []);
	useEffect(() => {
		if (query.error) handleAccessError(query.error);
	}, [handleAccessError, query.error]);
	useEffect(() => {
		if (
			editingID !== undefined &&
			query.data &&
			!query.data.items.some((alert) => alert.id === editingID)
		) {
			resetEditor();
		}
	}, [editingID, query.data, resetEditor]);
	const refresh = () => {
		void queryClient.invalidateQueries({
			queryKey: getListPriceAlertsQueryKey(symbol),
		});
		invalidateFavoriteQueries(queryClient);
	};
	const complete = () => {
		resetEditor();
		refresh();
	};
	const failed = (error: unknown) => {
		handleAccessError(error);
		notifications.show({
			color: "red",
			message: mutationErrorMessage(error),
			title: "Price alert failed",
		});
	};
	const createMutation = useCreatePriceAlert({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: complete },
	});
	const updateMutation = useUpdatePriceAlert({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: complete },
	});
	const deleteMutation = useDeletePriceAlert({
		fetch: telegramRequestOptions(),
		mutation: {
			onError: failed,
			onSuccess: (_response, variables) => {
				if (editingID === variables.alertId) resetEditor();
				refresh();
			},
		},
	});
	const isSaving = createMutation.isPending || updateMutation.isPending;
	const limitReached =
		query.data !== undefined && query.data.count >= query.data.limit;
	const submit = (event: FormEvent) => {
		event.preventDefault();
		const normalized = normalizePriceTarget(target);
		if ("error" in normalized) {
			setTargetError(normalized.error);
			return;
		}
		setTargetError(undefined);
		if (editingID !== undefined) {
			updateMutation.mutate({
				alertId: editingID,
				data: { target: normalized.value },
			});
			return;
		}
		createMutation.mutate({ symbol, data: { target: normalized.value } });
	};

	return (
		<Paper component="section" p={{ base: "xs", sm: "md" }}>
			<Stack gap="md">
				<div>
					<Title order={2} size="h4">
						Price alerts
					</Title>
					<Text c="dimmed" size="sm">
						One Telegram message is attempted when a live trade touches or
						crosses the target.
					</Text>
				</div>
				<PriceAlertList
					alerts={query.data?.items ?? []}
					deletingID={
						deleteMutation.isPending
							? deleteMutation.variables?.alertId
							: undefined
					}
					onDelete={(alertId) => deleteMutation.mutate({ alertId })}
					onEdit={(alert) => {
						setEditingID(alert.id);
						setTarget(alert.target);
						setTargetError(undefined);
					}}
				/>
				{query.isPending ? <Text c="dimmed">Loading alerts…</Text> : null}
				{query.isError ? (
					<Text c="red" size="sm">
						Unable to load price alerts.
					</Text>
				) : null}
				<PriceAlertForm
					isEditing={editingID !== undefined}
					isSaving={isSaving}
					limit={query.data?.limit}
					limitReached={limitReached}
					onCancel={resetEditor}
					onSubmit={submit}
					onTargetChange={setTarget}
					target={target}
					targetError={targetError}
				/>
			</Stack>
		</Paper>
	);
}
