import { Loader, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
	getAnalyzeMarketQueryKey,
	getListChartIndicatorsQueryKey,
	getListScannerIndicatorsQueryKey,
	useCreateScannerIndicator,
	useDeleteScannerIndicator,
	useListIndicatorTypes,
	useListScannerIndicators,
	useReorderScannerIndicators,
	useUpdateScannerIndicator,
} from "@/api/generated/api";
import type { ScannerIndicator } from "@/api/generated/models";
import { telegramRequestOptions } from "@/app/telegram";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";
import { ScannerIndicatorForm } from "@/features/scanner-settings/scanner-indicator-form";
import { ScannerIndicatorList } from "@/features/scanner-settings/scanner-indicator-list";
import { ScannerIndicatorRemovalConfirmation } from "@/features/scanner-settings/scanner-indicator-removal-confirmation";
import {
	mutationErrorMessage,
	orderIndicators,
} from "@/features/scanner-settings/utils";

export function ScannerSettings() {
	const queryClient = useQueryClient();
	const [removing, setRemoving] = useState<ScannerIndicator>();
	const types = useListIndicatorTypes({
		fetch: telegramRequestOptions(),
		query: {
			retry: false,
			select: (response) => response.data.items,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	const indicators = useListScannerIndicators({
		fetch: telegramRequestOptions(),
		query: { retry: false, select: (response) => response.data.items },
	});
	// Indicators change charts and table columns everywhere. The returned
	// promise settles once the list is current again.
	const refresh = () => {
		const list = queryClient.invalidateQueries({
			queryKey: getListScannerIndicatorsQueryKey(),
		});
		void queryClient.invalidateQueries({
			queryKey: getListChartIndicatorsQueryKey(),
		});
		void queryClient.invalidateQueries({
			queryKey: getAnalyzeMarketQueryKey().slice(0, 2),
		});
		invalidateFavoriteQueries(queryClient);
		return list;
	};
	const failed = (error: unknown) => {
		notifications.show({
			color: "red",
			message: mutationErrorMessage(error),
			title: "Indicator change failed",
		});
	};
	const createMutation = useCreateScannerIndicator({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: refresh },
	});
	const updateMutation = useUpdateScannerIndicator({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: refresh },
	});
	// The dragged order shows until the refreshed list arrives.
	const reorderMutation = useReorderScannerIndicators({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: refresh },
	});
	const deleteMutation = useDeleteScannerIndicator({
		fetch: telegramRequestOptions(),
		mutation: {
			onError: failed,
			onSettled: () => setRemoving(undefined),
			onSuccess: refresh,
		},
	});

	if (types.isPending || indicators.isPending) {
		return <Loader aria-label="Loading scanner settings" />;
	}
	if (types.isError || indicators.isError) {
		return (
			<Text c="red" size="sm">
				Scanner settings could not be loaded.
			</Text>
		);
	}
	return (
		<Stack gap="md">
			<ScannerIndicatorForm
				isSaving={createMutation.isPending}
				onSubmit={(data, reset) =>
					createMutation.mutate({ data }, { onSuccess: reset })
				}
				types={types.data}
			/>
			<ScannerIndicatorList
				disabled={
					updateMutation.isPending ||
					deleteMutation.isPending ||
					reorderMutation.isPending
				}
				indicators={orderIndicators(
					indicators.data,
					reorderMutation.isPending
						? reorderMutation.variables.data.ids
						: undefined,
				)}
				onDelete={setRemoving}
				onReorder={(ids) => reorderMutation.mutate({ data: { ids } })}
				onShowInTableChange={(indicator, showInTable) =>
					updateMutation.mutate({
						data: { scale: indicator.scale, show_in_table: showInTable },
						indicatorId: indicator.id,
					})
				}
			/>
			<ScannerIndicatorRemovalConfirmation
				isPending={deleteMutation.isPending}
				onCancel={() => setRemoving(undefined)}
				onConfirm={() => {
					if (removing) deleteMutation.mutate({ indicatorId: removing.id });
				}}
				title={removing?.title}
			/>
		</Stack>
	);
}
