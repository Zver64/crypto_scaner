import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import {
	getAnalyzeFavoritesQueryKey,
	getAnalyzeMarketQueryKey,
	getListChartIndicatorsQueryKey,
	getListScannerIndicatorsQueryKey,
	type listScannerIndicatorsResponseSuccess,
	useUpdateScannerIndicator,
} from "@/api/generated/api";
import type { ScannerIndicator } from "@/api/generated/models";
import { mutationErrorMessage } from "@/features/scanner-settings/utils";

export function useSaveScannerIndicator(indicator: ScannerIndicator) {
	const queryClient = useQueryClient();
	return useUpdateScannerIndicator({
		mutation: {
			onError: (error) => {
				notifications.show({
					color: "red",
					message: mutationErrorMessage(error),
					title: "Indicator change failed",
				});
			},
			onSuccess: async (response) => {
				// A list request started before this save must not overwrite it.
				await queryClient.cancelQueries({
					queryKey: getListScannerIndicatorsQueryKey(),
				});
				queryClient.setQueryData<listScannerIndicatorsResponseSuccess>(
					getListScannerIndicatorsQueryKey(),
					(current) =>
						current && {
							...current,
							data: {
								...current.data,
								items: current.data.items.map((indicator) =>
									indicator.id === response.data.id ? response.data : indicator,
								),
							},
						},
				);
				// Visibility and scale do not change other indicators or strategy
				// variables. Only dependent views become stale, without refetching.
				if (indicator.show_in_chart || response.data.show_in_chart) {
					void queryClient.invalidateQueries({
						queryKey: getListChartIndicatorsQueryKey({
							interval: response.data.interval,
						}),
						refetchType: "none",
					});
				}
				if (indicator.show_in_table !== response.data.show_in_table) {
					void queryClient.invalidateQueries({
						queryKey: getAnalyzeMarketQueryKey().slice(0, 2),
						refetchType: "none",
					});
					void queryClient.invalidateQueries({
						queryKey: getAnalyzeFavoritesQueryKey().slice(0, 2),
						refetchType: "none",
					});
				}
			},
		},
	});
}
