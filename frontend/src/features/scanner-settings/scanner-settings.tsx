import { Loader, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import type { ErrorType } from "@/api/fetch";
import {
	type listIndicatorTypesResponseSuccess,
	type listScannerIndicatorsResponseSuccess,
	useCreateScannerIndicator,
	useDeleteScannerIndicator,
	useListIndicatorTypes,
	useListScannerIndicators,
	useReorderScannerIndicators,
} from "@/api/generated/api";
import type { ErrorResponse, ScannerIndicator } from "@/api/generated/models";
import { SidebarLayout } from "@/components/sidebar-layout";
import { invalidateScannerIndicatorQueries } from "@/features/scanner-settings/query-cache";
import { ScannerIndicatorForm } from "@/features/scanner-settings/scanner-indicator-form";
import { ScannerIndicatorList } from "@/features/scanner-settings/scanner-indicator-list";
import { ScannerIndicatorRemovalConfirmation } from "@/features/scanner-settings/scanner-indicator-removal-confirmation";
import {
	mutationErrorMessage,
	orderIndicators,
} from "@/features/scanner-settings/utils";

const selectIndicatorTypes = (response: listIndicatorTypesResponseSuccess) =>
	response.data.items;

const selectIndicators = (response: listScannerIndicatorsResponseSuccess) =>
	response.data.items;

export function ScannerSettings() {
	const queryClient = useQueryClient();
	const [removing, setRemoving] = useState<ScannerIndicator>();
	const types = useListIndicatorTypes({
		query: {
			retry: false,
			select: selectIndicatorTypes,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	const indicators = useListScannerIndicators({
		query: { retry: false, select: selectIndicators },
	});
	const refresh = () => invalidateScannerIndicatorQueries(queryClient);
	const failed = (error: ErrorType<ErrorResponse>) => {
		notifications.show({
			color: "red",
			message: mutationErrorMessage(error),
			title: "Indicator change failed",
		});
	};
	const createMutation = useCreateScannerIndicator({
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
		<SidebarLayout
			gap="md"
			sidebar={
				<ScannerIndicatorForm
					isSaving={createMutation.isPending}
					onSubmit={(data) => createMutation.mutate({ data })}
					types={types.data}
				/>
			}
			sidebarPosition="start"
		>
			<Stack gap="md">
				<ScannerIndicatorList
					disabled={deleteMutation.isPending || reorderMutation.isPending}
					indicators={orderIndicators(
						indicators.data,
						reorderMutation.isPending
							? reorderMutation.variables.data.ids
							: undefined,
					)}
					onDelete={setRemoving}
					onReorder={(ids) => reorderMutation.mutate({ data: { ids } })}
				/>
				<ScannerIndicatorRemovalConfirmation
					isPending={deleteMutation.isPending}
					onCancel={() => setRemoving(undefined)}
					onConfirm={() => {
						if (removing) deleteMutation.mutate({ indicatorId: removing.id });
					}}
					subject={removing?.title}
				/>
			</Stack>
		</SidebarLayout>
	);
}
