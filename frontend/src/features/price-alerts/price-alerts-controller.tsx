import { Group, Paper, Stack, Text, Title } from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { ErrorType } from "@/api/fetch";
import {
	getListPriceAlertsQueryKey,
	type listPriceAlertsResponseSuccess,
	useCreatePriceAlert,
	useDeletePriceAlert,
	useListPriceAlerts,
	useUpdatePriceAlert,
} from "@/api/generated/api";
import type { ErrorResponse } from "@/api/generated/models";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";
import { scopedUserQueryKey } from "@/features/favorites/user-query-scope";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { PriceAlertForm } from "@/features/price-alerts/price-alert-form";
import { PriceAlertList } from "@/features/price-alerts/price-alert-list";
import { normalizePriceTarget } from "@/features/price-alerts/target";
import type { PriceAlertEditor } from "@/features/price-alerts/types";
import { describeApiError } from "@/utils/api-error";

function mutationErrorMessage(error: ErrorType<ErrorResponse> | null): string {
	return describeApiError(error, {
		fallback: "The price alert could not be saved.",
		messages: {
			access_denied: "Your Telegram account no longer has access.",
			alert_limit:
				"The maximum number of alerts for this instrument has been reached.",
			alert_not_found: "This alert no longer exists.",
			duplicate_target: "An alert with this target already exists.",
			symbol_not_found: "This instrument is no longer active.",
		},
	});
}

const selectAlerts = (response: listPriceAlertsResponseSuccess) =>
	response.data;

const emptyEditor: PriceAlertEditor = { editingID: undefined, target: "" };

interface PriceAlertsControllerProps {
	currentPrice?: ReactNode;
	priceSource?: CoinChartData;
	scope: string;
	symbol: string;
}

export function PriceAlertsController({
	currentPrice,
	priceSource,
	scope,
	symbol,
}: PriceAlertsControllerProps) {
	const queryClient = useQueryClient();
	const query = useListPriceAlerts(symbol, {
		query: {
			meta: { userScoped: true },
			queryKey: scopedUserQueryKey(getListPriceAlertsQueryKey(symbol), scope),
			refetchInterval: 15_000,
			refetchOnMount: "always",
			retry: false,
			select: selectAlerts,
		},
	});
	// Controlled, so the submit button and the percent change follow the
	// target as it is typed. The error stays until the next submit.
	const form = useForm<PriceAlertEditor>({
		clearInputErrorOnChange: false,
		initialValues: emptyEditor,
		mode: "controlled",
		validate: { target: (target) => normalizePriceTarget(target).error },
	});
	// An edited alert that no longer exists leaves the editor empty.
	const editedAlertGone =
		form.values.editingID !== undefined &&
		query.data !== undefined &&
		!query.data.items.some((alert) => alert.id === form.values.editingID);
	const editor = editedAlertGone ? emptyEditor : form.values;
	const targetError = editedAlertGone ? undefined : form.errors.target;
	const refresh = () => {
		void queryClient.invalidateQueries({
			queryKey: getListPriceAlertsQueryKey(symbol),
		});
		invalidateFavoriteQueries(queryClient);
	};
	const complete = () => {
		form.reset();
		refresh();
	};
	const failed = (error: ErrorType<ErrorResponse>) => {
		notifications.show({
			color: "red",
			message: mutationErrorMessage(error),
			title: "Price alert failed",
		});
	};
	const createMutation = useCreatePriceAlert({
		mutation: {
			meta: { userScoped: true },
			onError: failed,
			onSuccess: complete,
		},
	});
	const updateMutation = useUpdatePriceAlert({
		mutation: {
			meta: { userScoped: true },
			onError: failed,
			onSuccess: complete,
		},
	});
	const deleteMutation = useDeletePriceAlert({
		mutation: {
			meta: { userScoped: true },
			onError: failed,
			onSuccess: (_response, variables) => {
				if (editor.editingID === variables.alertId) form.reset();
				refresh();
			},
		},
	});
	const isSaving = createMutation.isPending || updateMutation.isPending;
	const limitReached =
		query.data !== undefined && query.data.count >= query.data.limit;
	// The form still holds an edited alert that no longer exists, although the
	// editor shows it empty; submitting it clears the form instead of
	// updating a missing alert.
	const submit = ({ editingID, target }: PriceAlertEditor) => {
		if (editedAlertGone) {
			form.reset();
			return;
		}
		const { value } = normalizePriceTarget(target);
		if (value === undefined) return;
		if (editingID !== undefined) {
			updateMutation.mutate({ alertId: editingID, data: { target: value } });
			return;
		}
		createMutation.mutate({ symbol, data: { target: value } });
	};

	return (
		<Paper component="section" p={{ base: "xs", sm: "md" }}>
			<Stack gap="md">
				<Group justify="space-between" wrap="nowrap">
					<Title order={2} size="h4">
						Price alerts
					</Title>
					{currentPrice}
				</Group>
				<PriceAlertList
					alerts={query.data?.items ?? []}
					deletingID={
						deleteMutation.isPending
							? deleteMutation.variables?.alertId
							: undefined
					}
					onDelete={(alertId) => deleteMutation.mutate({ alertId })}
					onEdit={(alert) => {
						form.setValues({ editingID: alert.id, target: alert.target });
						form.clearErrors();
					}}
					priceSource={priceSource}
				/>
				{query.isPending ? <Text c="dimmed">Loading alerts…</Text> : null}
				{query.isError ? (
					<Text c="red" size="sm">
						Unable to load price alerts.
					</Text>
				) : null}
				<PriceAlertForm
					isEditing={editor.editingID !== undefined}
					isSaving={isSaving}
					limit={query.data?.limit}
					limitReached={limitReached}
					onCancel={form.reset}
					onSubmit={form.onSubmit(submit)}
					// Not getInputProps: typing over an edited alert that no longer
					// exists also drops its id, so the form holds what the editor shows.
					onTargetChange={(target) => {
						form.setValues({ ...editor, target });
						if (editedAlertGone) form.clearErrors();
					}}
					priceSource={priceSource}
					target={editor.target}
					targetError={targetError}
				/>
			</Stack>
		</Paper>
	);
}
