import { Button, Group, Loader, Stack, Text, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
	getAnalyzeMarketQueryKey,
	getListChartIndicatorsQueryKey,
	getListScannerIndicatorsQueryKey,
	getListStrategiesQueryKey,
	getListStrategyVariablesQueryKey,
	useCreateScannerIndicator,
	useDeleteScannerIndicator,
	useDeleteUnusedScannerIndicators,
	useListIndicatorTypes,
	useListScannerIndicators,
	useReorderScannerIndicators,
	useUpdateScannerIndicator,
} from "@/api/generated/api";
import type { ScannerIndicator } from "@/api/generated/models";
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
	const [clearing, setClearing] = useState(false);
	const types = useListIndicatorTypes({
		query: {
			retry: false,
			select: (response) => response.data.items,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	const indicators = useListScannerIndicators({
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
		// Strategies read indicators as variables.
		void queryClient.invalidateQueries({
			queryKey: getListStrategyVariablesQueryKey(),
		});
		void queryClient.invalidateQueries({
			queryKey: getListStrategiesQueryKey(),
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
		mutation: { onError: failed, onSuccess: refresh },
	});
	const updateMutation = useUpdateScannerIndicator({
		mutation: { onError: failed, onSuccess: refresh },
	});
	// The dragged order shows until the refreshed list arrives.
	const reorderMutation = useReorderScannerIndicators({
		mutation: { onError: failed, onSuccess: refresh },
	});
	const deleteMutation = useDeleteScannerIndicator({
		mutation: {
			onError: failed,
			onSettled: () => setRemoving(undefined),
			onSuccess: refresh,
		},
	});

	const clearMutation = useDeleteUnusedScannerIndicators({
		mutation: {
			onError: failed,
			onSettled: () => setClearing(false),
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
				onSubmit={(data) => createMutation.mutate({ data })}
				types={types.data}
			/>
			<Group justify="space-between">
				<Title order={2} size="h4">
					Indicators
				</Title>
				{indicators.data.some(({ strategies }) => strategies.length === 0) ? (
					<Button
						color="red"
						disabled={clearMutation.isPending}
						onClick={() => setClearing(true)}
						size="compact-sm"
						variant="subtle"
					>
						Remove unused
					</Button>
				) : null}
			</Group>
			<ScannerIndicatorList
				disabled={
					updateMutation.isPending ||
					deleteMutation.isPending ||
					reorderMutation.isPending ||
					clearMutation.isPending
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
				subject={removing?.title}
			/>
			<ScannerIndicatorRemovalConfirmation
				isPending={clearMutation.isPending}
				onCancel={() => setClearing(false)}
				onConfirm={() => clearMutation.mutate()}
				subject={clearing ? "unused indicators" : undefined}
			/>
		</Stack>
	);
}
